<script lang="ts">
  import { tick } from "svelte";

  interface Option {
    value: string | number;
    label: string;
    disabled?: boolean;
  }

  interface Props {
    value: string | number;
    options: Option[];
    label?: string;
    /** Accessible name when there is no visible label. */
    ariaLabel?: string;
    /** Text shown when no option matches the value. */
    placeholder?: string;
    error?: string;
    disabled?: boolean;
    /** "sunken" sits inside forms (default); "raised" sits on sunken panels. */
    variant?: "sunken" | "raised";
    onchange: (val: any) => void;
  }

  let {
    value,
    options,
    label,
    ariaLabel,
    placeholder = "",
    error,
    disabled = false,
    variant = "sunken",
    onchange,
  }: Props = $props();

  const LIST_GAP_PX = 4;
  const LIST_MAX_HEIGHT_PX = 280;
  const VIEWPORT_MARGIN_PX = 8;

  const uid = Math.random().toString(36).slice(2, 8);
  const triggerId = `select-${uid}`;
  const listId = `${triggerId}-list`;
  const labelId = `${triggerId}-label`;
  const errorId = `${triggerId}-error`;

  let open = $state(false);
  let activeIndex = $state(-1);
  let trigger = $state<HTMLButtonElement | undefined>();
  let list = $state<HTMLUListElement | undefined>();
  let listStyle = $state("");

  const selected = $derived(options.find((o) => String(o.value) === String(value)));

  function positionList() {
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    const spaceBelow = window.innerHeight - rect.bottom - VIEWPORT_MARGIN_PX;
    const spaceAbove = rect.top - VIEWPORT_MARGIN_PX;
    const openUp = spaceBelow < Math.min(LIST_MAX_HEIGHT_PX, 160) && spaceAbove > spaceBelow;
    const maxHeight = Math.min(LIST_MAX_HEIGHT_PX, openUp ? spaceAbove : spaceBelow);
    const vertical = openUp
      ? `bottom: ${window.innerHeight - rect.top + LIST_GAP_PX}px;`
      : `top: ${rect.bottom + LIST_GAP_PX}px;`;
    listStyle = `${vertical} left: ${rect.left}px; width: ${rect.width}px; max-height: ${maxHeight}px;`;
  }

  async function openList() {
    if (disabled || open) return;
    positionList();
    const current = options.findIndex((o) => String(o.value) === String(value));
    activeIndex = current >= 0 ? current : nextEnabled(-1, 1);
    open = true;
    await tick();
    scrollActiveIntoView();
  }

  function closeList(focusTrigger = true) {
    open = false;
    if (focusTrigger) trigger?.focus();
  }

  function choose(index: number) {
    const option = options[index];
    if (!option || option.disabled) return;
    closeList();
    if (String(option.value) !== String(value)) onchange(option.value);
  }

  function nextEnabled(from: number, step: 1 | -1): number {
    for (let i = from + step; i >= 0 && i < options.length; i += step) {
      if (!options[i].disabled) return i;
    }
    return from;
  }

  function scrollActiveIntoView() {
    list?.querySelector<HTMLElement>(`[data-index="${activeIndex}"]`)?.scrollIntoView?.({
      block: "nearest",
    });
  }

  function typeAhead(key: string) {
    const lower = key.toLowerCase();
    const start = activeIndex + 1;
    const ordered = [...options.slice(start), ...options.slice(0, start)];
    const match = ordered.find((o) => !o.disabled && o.label.toLowerCase().startsWith(lower));
    if (match) activeIndex = options.indexOf(match);
  }

  async function handleKeydown(e: KeyboardEvent) {
    if (disabled) return;
    if (!open) {
      if (["ArrowDown", "ArrowUp", "Enter", " "].includes(e.key)) {
        e.preventDefault();
        await openList();
      }
      return;
    }
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        activeIndex = nextEnabled(activeIndex, 1);
        break;
      case "ArrowUp":
        e.preventDefault();
        activeIndex = nextEnabled(activeIndex, -1);
        break;
      case "Home":
        e.preventDefault();
        activeIndex = nextEnabled(-1, 1);
        break;
      case "End":
        e.preventDefault();
        activeIndex = nextEnabled(options.length, -1);
        break;
      case "Enter":
      case " ":
        e.preventDefault();
        choose(activeIndex);
        return;
      case "Escape":
        e.preventDefault();
        e.stopPropagation();
        closeList();
        return;
      case "Tab":
        closeList(false);
        return;
      default:
        if (e.key.length === 1) typeAhead(e.key);
    }
    await tick();
    scrollActiveIntoView();
  }

  function handleWindowPointer(e: PointerEvent) {
    if (!open) return;
    const target = e.target as Node;
    if (trigger?.contains(target) || list?.contains(target)) return;
    closeList(false);
  }

  function handleWindowScroll(e: Event) {
    if (!open || list?.contains(e.target as Node)) return;
    closeList(false);
  }
</script>

<svelte:window
  onpointerdown={handleWindowPointer}
  onresize={() => open && closeList(false)}
  onscrollcapture={handleWindowScroll}
/>

<div class="select-group">
  {#if label}
    <span class="select-label" id={labelId}>{label}</span>
  {/if}
  <button
    bind:this={trigger}
    id={triggerId}
    type="button"
    class="select-trigger {variant}"
    class:invalid={Boolean(error)}
    class:open
    {disabled}
    role="combobox"
    aria-haspopup="listbox"
    aria-expanded={open}
    aria-controls={listId}
    aria-labelledby={label ? `${labelId} ${triggerId}` : undefined}
    aria-label={label ? undefined : ariaLabel}
    aria-invalid={Boolean(error)}
    aria-describedby={error ? errorId : undefined}
    aria-activedescendant={open && activeIndex >= 0 ? `${listId}-${activeIndex}` : undefined}
    onclick={() => (open ? closeList() : openList())}
    onkeydown={handleKeydown}
  >
    <span class="select-value" class:placeholder={!selected}>
      {selected ? selected.label : placeholder}
    </span>
    <svg class="chevron" viewBox="0 0 16 16" aria-hidden="true">
      <path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.6" />
    </svg>
  </button>
  {#if error}
    <p id={errorId} class="select-error" role="alert">{error}</p>
  {/if}

  {#if open}
    <ul
      bind:this={list}
      id={listId}
      class="select-list"
      role="listbox"
      aria-labelledby={label ? labelId : undefined}
      aria-label={label ? undefined : ariaLabel}
      style={listStyle}
    >
      {#each options as opt, i (opt.value)}
        <!-- keyboard navigation lives on the combobox trigger (aria-activedescendant) -->
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <li
          id={`${listId}-${i}`}
          data-index={i}
          role="option"
          class="select-option"
          class:active={i === activeIndex}
          class:selected={String(opt.value) === String(value)}
          aria-selected={String(opt.value) === String(value)}
          aria-disabled={opt.disabled || undefined}
          onpointerenter={() => !opt.disabled && (activeIndex = i)}
          onpointerdown={(e) => e.preventDefault()}
          onclick={() => choose(i)}
        >
          <span class="option-label">{opt.label}</span>
          {#if String(opt.value) === String(value)}
            <svg class="check" viewBox="0 0 16 16" aria-hidden="true">
              <path d="M3.5 8.5l3 3 6-7" fill="none" stroke="currentColor" stroke-width="1.8" />
            </svg>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .select-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    width: 100%;
    min-width: 0;
  }

  .select-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .select-trigger {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    width: 100%;
    min-width: 0;
    height: 36px;
    padding: 0 var(--space-3);
    border-radius: var(--radius-md);
    border: none;
    color: var(--ink);
    font-family: var(--font-sans);
    font-size: var(--text-sm);
    text-align: left;
    outline: none;
    cursor: pointer;
    transition: box-shadow var(--duration-fast) var(--easing-out);
  }

  .select-trigger.sunken {
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .select-trigger.raised {
    background-color: var(--surface);
    box-shadow: var(--elev-1);
  }

  .select-trigger:focus-visible,
  .select-trigger.open {
    box-shadow: var(--focus-ring);
  }

  .select-trigger.invalid {
    box-shadow: 0 0 0 1px var(--dot-conflict);
  }

  .select-trigger:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .select-value {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .select-value.placeholder {
    color: var(--text-subtle);
  }

  .chevron {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
    color: var(--text-muted);
    transition: transform var(--duration-fast) var(--easing-out);
  }

  .select-trigger.open .chevron {
    transform: rotate(180deg);
  }

  .select-error {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--dot-conflict);
  }

  .select-list {
    position: fixed;
    z-index: 1000;
    margin: 0;
    padding: var(--space-1);
    list-style: none;
    overflow-y: auto;
    background-color: var(--surface);
    border-radius: var(--radius-md);
    box-shadow: var(--elev-3);
  }

  .select-option {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    min-height: 32px;
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--text-sm);
    color: var(--ink);
    cursor: pointer;
  }

  .select-option.active {
    background-color: var(--surface-sunken);
  }

  .select-option.selected {
    font-weight: var(--weight-medium);
  }

  .select-option[aria-disabled="true"] {
    color: var(--text-subtle);
    cursor: default;
  }

  .option-label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .check {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
    color: var(--accent);
  }
</style>
