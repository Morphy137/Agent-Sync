<script lang="ts" module>
  export interface NotificationAction {
    label: string;
    primary?: boolean;
    onclick: () => void;
  }

  export interface AppNotification {
    /** Stable id per situation, so a closed popup stays closed while the situation lasts. */
    id: string;
    variant: "info" | "warning";
    title: string;
    message: string;
    actions?: NotificationAction[];
  }
</script>

<script lang="ts">
  import { Bell, X } from "@lucide/svelte";
  import { t } from "../../i18n";
  import Button from "../Button.svelte";
  import IconButton from "../IconButton.svelte";
  import { portal } from "../../actions/portal";

  interface Props {
    items: AppNotification[];
  }

  let { items }: Props = $props();

  let panelOpen = $state(false);
  let container = $state<HTMLDivElement | undefined>();
  /** Popups the user closed; they stay listed under the bell. */
  let closedPopups = $state<string[]>([]);

  const popup = $derived(items.find((item) => !closedPopups.includes(item.id)));

  $effect(() => {
    // Forget closed popups whose situation is gone, so they show again if it comes back.
    const activeIds = items.map((item) => item.id);
    const stillActive = closedPopups.filter((id) => activeIds.includes(id));
    if (stillActive.length !== closedPopups.length) closedPopups = stillActive;
  });

  function closePopup(id: string) {
    closedPopups = [...closedPopups, id];
  }

  function runAction(item: AppNotification, action: NotificationAction) {
    closePopup(item.id);
    panelOpen = false;
    action.onclick();
  }

  function handleWindowPointer(e: PointerEvent) {
    if (panelOpen && container && !container.contains(e.target as Node)) panelOpen = false;
  }

  function handleWindowKey(e: KeyboardEvent) {
    if (panelOpen && e.key === "Escape") panelOpen = false;
  }
</script>

<svelte:window onpointerdown={handleWindowPointer} onkeydown={handleWindowKey} />

{#snippet notificationBody(item: AppNotification, inPopup: boolean)}
  <div class="item-head">
    <span class="dot dot-{item.variant}" aria-hidden="true"></span>
    <span class="item-title">{item.title}</span>
    {#if inPopup}
      <IconButton label={t("notifications.close")} onclick={() => closePopup(item.id)}>
        <X size={14} />
      </IconButton>
    {/if}
  </div>
  <p class="item-message">{item.message}</p>
  {#if item.actions?.length}
    <div class="item-actions">
      {#each item.actions as action (action.label)}
        <Button
          size="sm"
          variant={action.primary ? "primary" : "secondary"}
          onclick={() => runAction(item, action)}
        >
          {action.label}
        </Button>
      {/each}
    </div>
  {/if}
{/snippet}

<div class="notification-center" bind:this={container}>
  <IconButton
    label={items.length > 0
      ? t("notifications.bell_count", { count: items.length })
      : t("notifications.bell")}
    ariaExpanded={panelOpen}
    onclick={() => (panelOpen = !panelOpen)}
  >
    <Bell size={16} />
  </IconButton>
  {#if items.length > 0}
    <span class="count" aria-hidden="true">{items.length}</span>
  {/if}

  {#if panelOpen}
    <div class="panel" role="dialog" aria-label={t("notifications.title")}>
      <div class="panel-title">{t("notifications.title")}</div>
      {#if items.length === 0}
        <p class="panel-empty">{t("notifications.empty")}</p>
      {:else}
        <ul class="panel-list">
          {#each items as item (item.id)}
            <li class="panel-item">{@render notificationBody(item, false)}</li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</div>

{#if popup && !panelOpen}
  <div class="popup" role="status" use:portal>
    {@render notificationBody(popup, true)}
  </div>
{/if}

<style>
  .notification-center {
    position: relative;
    display: flex;
    align-items: center;
  }

  .count {
    position: absolute;
    top: 2px;
    right: 2px;
    min-width: 15px;
    height: 15px;
    padding: 0 4px;
    border-radius: var(--radius-xs);
    background-color: var(--accent);
    color: var(--accent-contrast);
    font-size: 10px;
    font-weight: var(--weight-semibold);
    line-height: 15px;
    text-align: center;
    pointer-events: none;
  }

  .panel,
  .popup {
    width: 340px;
    background-color: var(--surface);
    border-radius: var(--radius-lg);
    box-shadow: var(--elev-3);
    z-index: 150;
  }

  .panel {
    position: absolute;
    top: calc(100% + var(--space-2));
    right: 0;
    padding: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .panel-title {
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .panel-empty {
    margin: 0;
    padding: var(--space-4) 0;
    text-align: center;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }

  .panel-list {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    max-height: 360px;
    overflow-y: auto;
  }

  .panel-item {
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .popup {
    position: fixed;
    top: calc(var(--titlebar-height) + var(--space-3));
    right: var(--space-4);
    padding: var(--space-3) var(--space-4);
    animation: popupEnter var(--duration-normal) var(--easing-out);
  }

  .item-head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: 28px;
  }

  .item-title {
    flex: 1;
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .item-message {
    margin: var(--space-1) 0 0;
    font-size: var(--text-xs);
    line-height: 1.5;
    color: var(--text-muted);
  }

  .item-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }

  .dot {
    width: 8px;
    height: 8px;
    flex-shrink: 0;
    border-radius: var(--radius-full);
  }

  .dot-info {
    background-color: var(--dot-down);
  }

  .dot-warning {
    background-color: var(--dot-up);
  }

  @keyframes popupEnter {
    from {
      opacity: 0;
      transform: translateY(-6px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .popup {
      animation: none;
    }
  }
</style>
