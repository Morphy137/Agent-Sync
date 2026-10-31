<script lang="ts">
  import { GitFork, FolderSync, Rss, MapPin } from "@lucide/svelte";
  import { t } from "../i18n";
  import type { AppView } from "../stores/appState.svelte";

  interface Props {
    value: AppView;
    onchange: (view: AppView) => void;
  }

  let { value, onchange }: Props = $props();

  const tabs: { id: AppView; labelKey: string; icon: typeof GitFork }[] = [
    { id: "rules", labelKey: "navigation.rules", icon: GitFork },
    { id: "connections", labelKey: "navigation.connections", icon: FolderSync },
    { id: "subscriptions", labelKey: "navigation.subscriptions", icon: Rss },
    { id: "locations", labelKey: "navigation.locations", icon: MapPin },
  ];

  let track = $state<HTMLDivElement | undefined>();
  const buttons: HTMLButtonElement[] = $state([]);
  let thumb = $state({ left: 0, width: 0, ready: false });

  /** Tabs have different widths, so the sliding thumb is measured from the active button. */
  function measure() {
    const active = buttons[tabs.findIndex((tab) => tab.id === value)];
    if (!active) return;
    thumb = { left: active.offsetLeft, width: active.offsetWidth, ready: true };
  }

  $effect(() => {
    void value;
    measure();
  });

  $effect(() => {
    if (!track || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => measure());
    observer.observe(track);
    return () => observer.disconnect();
  });

  function handleKeyDown(e: KeyboardEvent, index: number) {
    const step = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
    if (step === 0) return;
    e.preventDefault();
    const next = tabs[(index + step + tabs.length) % tabs.length];
    onchange(next.id);
  }
</script>

<div class="view-tabs" role="tablist" aria-label={t("navigation.aria_label")} bind:this={track}>
  <span
    class="tab-thumb"
    class:ready={thumb.ready}
    style="transform: translateX({thumb.left}px); width: {thumb.width}px;"
    aria-hidden="true"
  ></span>
  {#each tabs as tab, index (tab.id)}
    {@const Icon = tab.icon}
    <button
      bind:this={buttons[index]}
      type="button"
      role="tab"
      class="view-tab"
      class:active={value === tab.id}
      aria-selected={value === tab.id}
      tabindex={value === tab.id ? 0 : -1}
      onclick={() => onchange(tab.id)}
      onkeydown={(e) => handleKeyDown(e, index)}
    >
      <Icon size={14} />
      <span>{t(tab.labelKey)}</span>
    </button>
  {/each}
</div>

<style>
  .view-tabs {
    position: relative;
    display: flex;
    gap: var(--space-1);
    padding: var(--space-1);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-md);
    align-self: flex-start;
    max-width: 100%;
    overflow-x: auto;
  }

  .tab-thumb {
    position: absolute;
    top: var(--space-1);
    bottom: var(--space-1);
    left: 0;
    border-radius: var(--radius-sm);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    pointer-events: none;
    opacity: 0;
  }

  .tab-thumb.ready {
    opacity: 1;
    transition:
      transform var(--duration-normal) var(--easing-out),
      width var(--duration-normal) var(--easing-out);
  }

  .view-tab {
    position: relative;
    z-index: 1;
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border: none;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    cursor: pointer;
    white-space: nowrap;
    transition:
      background-color var(--duration-fast) var(--easing-out),
      color var(--duration-fast) var(--easing-out);
  }

  .view-tab:hover {
    color: var(--ink);
  }

  .view-tab.active {
    color: var(--ink);
  }

  .view-tab:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }
  @media (prefers-reduced-motion: reduce) {
    .tab-thumb.ready {
      transition: none;
    }
  }
</style>
