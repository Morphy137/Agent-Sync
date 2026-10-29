<script lang="ts">
  interface Props {
    status: string;
    text: string;
  }

  let { status, text }: Props = $props();

  const dotClass = $derived.by(() => {
    switch (status) {
      case "in_sync":
      case "ok":
        return "dot-ok";
      case "source_changed":
      case "new_in_source":
      case "target_missing":
      case "down":
        return "dot-down";
      case "target_changed":
      case "diverged":
      case "untracked_diff":
      case "up":
        return "dot-up";
      case "conflict":
      case "target_unavailable":
      case "unreadable":
        return "dot-conflict";
      default:
        return "dot-idle";
    }
  });
</script>

<span class="status-pill">
  <span class="dot {dotClass}"></span>
  <span class="label">{text}</span>
</span>

<style>
  .status-pill {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: 3px var(--space-3);
    border-radius: var(--radius-full);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    user-select: none;
    line-height: 1.3;
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: var(--radius-full);
    flex-shrink: 0;
  }

  .dot-ok {
    background-color: var(--dot-ok);
  }

  .dot-down {
    background-color: var(--dot-down);
  }

  .dot-up {
    background-color: var(--dot-up);
  }

  .dot-conflict {
    background-color: var(--dot-conflict);
  }

  .dot-idle {
    background-color: var(--dot-idle);
  }
</style>
