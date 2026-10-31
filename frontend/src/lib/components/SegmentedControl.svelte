<script lang="ts">
  interface SegmentOption {
    value: string;
    label: string;
  }

  interface Props {
    options: SegmentOption[];
    value: string;
    label?: string;
    ariaLabel?: string;
    size?: "sm" | "md";
    onchange: (val: string) => void;
  }

  let { options, value, label, ariaLabel, size = "md", onchange }: Props = $props();
  const buttons: HTMLButtonElement[] = $state([]);

  const activeIndex = $derived(
    Math.max(
      0,
      options.findIndex((o) => o.value === value)
    )
  );
  const count = $derived(Math.max(1, options.length));

  function handleKeydown(event: KeyboardEvent, index: number) {
    const step = ["ArrowRight", "ArrowDown"].includes(event.key)
      ? 1
      : ["ArrowLeft", "ArrowUp"].includes(event.key)
        ? -1
        : 0;
    if (!step || options.length === 0) return;
    event.preventDefault();
    const next = (index + step + options.length) % options.length;
    onchange(options[next].value);
    buttons[next]?.focus();
  }
</script>

<div class="segmented-wrapper">
  {#if label}
    <span class="segmented-label">{label}</span>
  {/if}
  <div
    class="segmented-track size-{size}"
    role="radiogroup"
    aria-label={label || ariaLabel}
    style="--count: {count}; --active-index: {activeIndex};"
  >
    <div class="segmented-thumb" aria-hidden="true"></div>
    {#each options as opt, index (opt.value)}
      <button
        bind:this={buttons[index]}
        type="button"
        role="radio"
        aria-checked={opt.value === value}
        tabindex={index === activeIndex ? 0 : -1}
        class="segmented-option"
        class:active={opt.value === value}
        onclick={() => onchange(opt.value)}
        onkeydown={(event) => handleKeydown(event, index)}
      >
        {opt.label}
      </button>
    {/each}
  </div>
</div>

<style>
  .segmented-wrapper {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    width: 100%;
  }

  .segmented-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .segmented-track {
    position: relative;
    display: grid;
    grid-template-columns: repeat(var(--count), 1fr);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-md);
    padding: 3px;
    user-select: none;
  }

  .size-sm {
    height: 30px;
    border-radius: var(--radius-sm);
  }

  .size-md {
    height: 36px;
  }

  .segmented-thumb {
    position: absolute;
    top: 3px;
    bottom: 3px;
    left: 3px;
    width: calc((100% - 6px) / var(--count));
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    border-radius: calc(var(--radius-md) - 2px);
    transform: translateX(calc(var(--active-index) * 100%));
    transition: transform var(--duration-normal) var(--easing-out);
    pointer-events: none;
  }

  .size-sm .segmented-thumb {
    border-radius: calc(var(--radius-sm) - 2px);
  }

  .segmented-option {
    position: relative;
    z-index: 1;
    border: none;
    background: transparent;
    color: var(--text-muted);
    font-family: var(--font-sans);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    cursor: pointer;
    padding: 0 var(--space-2);
    border-radius: calc(var(--radius-md) - 2px);
    transition: color var(--duration-fast) var(--easing-out);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .segmented-option.active {
    color: var(--ink);
    font-weight: var(--weight-semibold);
  }

  .segmented-option:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  @media (prefers-reduced-motion: reduce) {
    .segmented-thumb {
      transition: none;
    }
  }
</style>
