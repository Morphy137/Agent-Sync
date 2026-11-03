<script lang="ts">
  import { ChevronRight } from "@lucide/svelte";
  import type { DotTone } from "./rules/ruleUi";
  import StatusDot from "./rules/StatusDot.svelte";
  import "./cardZones.css";

  interface Props {
    expanded: boolean;
    /** Accessible name; describes the action ("Show details for ..."). */
    label: string;
    /** Number shown next to the chevron (files, destinations...). Omitted when unknown. */
    count?: number;
    /** Tooltip; defaults to the label. */
    title?: string;
    /** Id of the region this button shows or hides. */
    controls?: string;
    /** Highlights hidden content that needs attention. */
    attention?: DotTone;
    onclick: () => void;
  }

  let { expanded, label, count, title, controls, attention, onclick }: Props = $props();
</script>

<button
  type="button"
  class="expand-toggle"
  class:expanded
  aria-label={label}
  title={title ?? label}
  aria-expanded={expanded}
  aria-controls={controls}
  {onclick}
>
  <ChevronRight size={14} aria-hidden="true" />
  {#if count !== undefined}<span class="expand-count">{count}</span>{/if}
  {#if attention && attention !== "ok" && attention !== "idle"}
    <span class="expand-attention"><StatusDot tone={attention} /></span>
  {/if}
</button>

<style>
  .expand-toggle {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 2px;
    min-width: var(--card-zone-expand);
    max-width: 100%;
    height: 28px;
    padding: 0 var(--space-1);
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    font-variant-numeric: tabular-nums;
    cursor: pointer;
    transition:
      background-color var(--duration-fast) var(--easing-out),
      color var(--duration-fast) var(--easing-out);
  }

  .expand-toggle:hover {
    background-color: var(--surface-sunken);
    color: var(--ink);
  }

  .expand-toggle:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .expand-toggle :global(svg) {
    flex-shrink: 0;
    transition: transform var(--duration-fast) var(--easing-out);
  }

  .expanded :global(svg) {
    transform: rotate(90deg);
  }

  .expand-attention {
    position: absolute;
    top: 2px;
    right: 2px;
    line-height: 0;
    pointer-events: none;
  }
</style>
