<script lang="ts">
  import { CloudDownload, FolderTree, GitBranch, RefreshCw, Rss, Trash2 } from "@lucide/svelte";
  import type { service } from "../../../../wailsjs/go/models";
  import { formatDate, t } from "../../i18n";
  import Button from "../Button.svelte";
  import IconButton from "../IconButton.svelte";
  import StatusPill from "../StatusPill.svelte";
  import Switch from "../Switch.svelte";
  import IncomingChangesList from "./IncomingChangesList.svelte";

  interface Props {
    subscription: service.SubscriptionInfo;
    checking?: boolean;
    updating?: boolean;
    oncheck: () => void;
    onupdate: () => void;
    onautoupdate: (enabled: boolean) => void;
    onbrowse: () => void;
    onremove: () => void;
    onviewdiff: (path: string) => void;
  }

  let {
    subscription,
    checking = false,
    updating = false,
    oncheck,
    onupdate,
    onautoupdate,
    onbrowse,
    onremove,
    onviewdiff,
  }: Props = $props();

  const update = $derived(subscription.update);
  const behind = $derived(update && !update.offline ? update.behind : 0);
  const incomingFiles = $derived(update?.incomingFiles ?? []);
  const busy = $derived(checking || updating);

  const badge = $derived.by(() => {
    if (!update) return { status: "idle", text: t("subscriptions.badge.not_checked") };
    if (update.offline) return { status: "up", text: t("subscriptions.badge.offline") };
    if (update.behind > 0) {
      return {
        status: "down",
        text:
          update.behind === 1
            ? t("subscriptions.badge.behind_one")
            : t("subscriptions.badge.behind_other", { count: update.behind }),
      };
    }
    return { status: "ok", text: t("subscriptions.badge.up_to_date") };
  });

  const ruleCountText = $derived.by(() => {
    if (subscription.ruleCount === 0) return t("subscriptions.card.rules_none");
    if (subscription.ruleCount === 1) return t("subscriptions.card.rules_one");
    return t("subscriptions.card.rules_other", { count: subscription.ruleCount });
  });

  function dateOrNever(value?: string): string {
    return value ? formatDate(value) : t("subscriptions.card.never");
  }
</script>

<article class="card" aria-labelledby="sub-title-{subscription.id}">
  <div class="card-top">
    <div class="tile" aria-hidden="true">
      <Rss size={18} />
    </div>
    <div class="info">
      <div class="title-row">
        <h3 class="name" id="sub-title-{subscription.id}">{subscription.name}</h3>
        <StatusPill status={badge.status} text={badge.text} />
      </div>
      <span class="url" title={subscription.url}>{subscription.url}</span>
      <div class="meta">
        {#if subscription.branch}
          <span class="branch">
            <GitBranch size={12} />
            <span title={subscription.branch}>{subscription.branch}</span>
          </span>
        {/if}
        <span class="rules" class:rules-empty={subscription.ruleCount === 0}>{ruleCountText}</span>
      </div>
    </div>
  </div>

  <dl class="dates">
    <div>
      <dt>{t("subscriptions.card.last_checked")}</dt>
      <dd>{dateOrNever(subscription.lastCheckedAt)}</dd>
    </div>
    <div>
      <dt>{t("subscriptions.card.last_updated")}</dt>
      <dd>{dateOrNever(subscription.lastUpdatedAt)}</dd>
    </div>
  </dl>

  {#if incomingFiles.length > 0}
    <IncomingChangesList files={incomingFiles} {onviewdiff} />
  {/if}

  <div class="auto-update">
    <Switch
      checked={subscription.autoUpdate}
      label={t("subscriptions.card.auto_update")}
      onchange={onautoupdate}
    />
    <p class="auto-hint">{t("subscriptions.card.auto_update_hint")}</p>
  </div>

  <div class="hairline" aria-hidden="true"></div>

  <div class="actions">
    <div class="actions-main">
      <Button
        size="sm"
        variant={behind > 0 ? "primary" : "secondary"}
        loading={updating}
        disabled={busy}
        onclick={onupdate}
      >
        <CloudDownload size={14} />
        <span>{t("subscriptions.card.update_now")}</span>
      </Button>
      <Button size="sm" variant="secondary" loading={checking} disabled={busy} onclick={oncheck}>
        {#if !checking}
          <RefreshCw size={14} />
        {/if}
        <span>{t("subscriptions.card.check")}</span>
      </Button>
      <Button size="sm" variant="ghost" onclick={onbrowse}>
        <FolderTree size={14} />
        <span>{t("subscriptions.card.browse")}</span>
      </Button>
    </div>
    <IconButton
      label={t("subscriptions.card.remove_for", { name: subscription.name })}
      disabled={busy}
      onclick={onremove}
    >
      <Trash2 size={16} />
    </IconButton>
  </div>
</article>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: var(--space-5);
    background-color: var(--surface);
    border-radius: var(--radius-lg);
    box-shadow: var(--elev-1);
    min-width: 0;
    transition: box-shadow var(--duration-fast) var(--easing-out);
  }

  .card:hover {
    box-shadow: var(--elev-2);
  }

  .card-top {
    display: flex;
    gap: var(--space-3);
    min-width: 0;
  }

  .tile {
    width: 40px;
    height: 40px;
    flex-shrink: 0;
    border-radius: var(--radius-md);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--ink);
  }

  .info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .title-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .name {
    margin: 0;
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .url {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-3);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .branch {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    font-family: var(--font-mono);
    min-width: 0;
    max-width: 100%;
  }

  .branch :global(svg) {
    flex-shrink: 0;
  }

  .branch span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .rules {
    font-variant-numeric: tabular-nums;
  }

  .rules-empty {
    color: var(--text-subtle);
  }

  .dates {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: var(--space-2) var(--space-4);
    margin: 0;
  }

  .dates div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .dates dt {
    font-size: 11px;
    color: var(--text-subtle);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .dates dd {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--ink);
    font-variant-numeric: tabular-nums;
  }

  .auto-update {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .auto-hint {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.5;
  }

  .hairline {
    height: 1px;
    background-color: var(--hairline);
    margin: 0 calc(-1 * var(--space-5));
  }

  .actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .actions-main {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
</style>
