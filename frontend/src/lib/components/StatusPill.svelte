<script lang="ts">
  interface Props {
    status: string;
    text: string;
  }

  let { status, text }: Props = $props();

  const variantClass = $derived.by(() => {
    switch (status) {
      case "in_sync":
        return "status-success";
      case "source_changed":
      case "new_in_source":
      case "target_missing":
        return "status-info";
      case "target_changed":
        return "status-accent";
      case "diverged":
      case "untracked_diff":
        return "status-warning";
      case "conflict":
      case "target_unavailable":
        return "status-danger";
      default:
        return "status-neutral";
    }
  });
</script>

<span class="status-pill {variantClass}">
  <span class="dot"></span>
  <span class="label">{text}</span>
</span>

<style>
  .status-pill {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-full);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    border: 1px solid transparent;
    user-select: none;
    line-height: 1.4;
  }

  .dot {
    width: 6px;
    height: 6px;
    border-radius: var(--radius-full);
    background-color: currentColor;
  }

  .status-success {
    background-color: var(--success-bg);
    color: var(--success-fg);
    border-color: var(--success-border);
  }

  .status-info {
    background-color: var(--info-bg);
    color: var(--info-fg);
    border-color: var(--info-border);
  }

  .status-accent {
    background-color: var(--surface-raised);
    color: var(--accent);
    border-color: var(--accent);
  }

  .status-warning {
    background-color: var(--warning-bg);
    color: var(--warning-fg);
    border-color: var(--warning-border);
  }

  .status-danger {
    background-color: var(--danger-bg);
    color: var(--danger-fg);
    border-color: var(--danger-border);
  }

  .status-neutral {
    background-color: var(--surface-raised);
    color: var(--text-muted);
    border-color: var(--border);
  }
</style>
