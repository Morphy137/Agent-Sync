<script lang="ts">
  import {
    CloudDownload,
    FolderTree,
    GitBranch,
    Lightbulb,
    RefreshCw,
    Rss,
    Trash2,
  } from "@lucide/svelte";
  import type { service } from "../../../../wailsjs/go/models";
  import { api } from "../../api";
  import { formatDate, t } from "../../i18n";
  import { repositoryPageUrl } from "../../repoUrl";
  import Button from "../Button.svelte";
  import ExpandToggle from "../ExpandToggle.svelte";
  import IconButton from "../IconButton.svelte";
  import StatusPill from "../StatusPill.svelte";
  import Switch from "../Switch.svelte";
  import Tooltip from "../Tooltip.svelte";
  import IncomingChangesList from "./IncomingChangesList.svelte";
  import "../cardZones.css";

  interface Props {
    subscription: service.SubscriptionInfo;
    checking?: boolean;
    updating?: boolean;
    oncheck: () => void;
    onupdate: () => void;
    onautoupdate: (enabled: boolean) => void;
    onbrowse: () => void;
    onsuggestions?: () => void;
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
    onsuggestions,
    onremove,
    onviewdiff,
  }: Props = $props();

  const update = $derived(subscription.update);
  const behind = $derived(update && !update.offline ? update.behind : 0);
  const incomingFiles = $derived(update?.incomingFiles ?? []);
  const busy = $derived(checking || updating);
  const suggestionsPending = $derived(subscription.suggestionsPending ?? 0);
  const suggestionsLabel = $derived(
    suggestionsPending === 1
      ? t("subscriptions.suggestions.open_count_one")
      : t("subscriptions.suggestions.open_count_other", { count: suggestionsPending })
  );
  const uid = $props.id();
  const incomingId = `incoming-${uid}`;
  let incomingOpen = $state(false);
  const incomingLabel = $derived(
    incomingFiles.length === 1
      ? t("subscriptions.incoming.toggle_one")
      : t("subscriptions.incoming.toggle_other", { count: incomingFiles.length })
  );

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

  const repositoryPage = $derived(repositoryPageUrl(subscription.url));
  const branchLabel = $derived(subscription.branch || t("subscriptions.card.repository"));

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
        <h3 class="name" id="sub-title-{subscription.id}" title={subscription.name}>
          {subscription.name}
        </h3>
        <StatusPill status={badge.status} text={badge.text} />
      </div>
      <div class="meta">
        {#if repositoryPage}
          <button
            type="button"
            class="branch branch-link"
            title={subscription.url}
            aria-label={`${t("subscriptions.card.open_repository")}: ${branchLabel}`}
            onclick={() => api.openURL(repositoryPage)}
          >
            <GitBranch size={12} />
            <span>{branchLabel}</span>
          </button>
        {:else}
          <span class="branch" title={subscription.url}>
            <GitBranch size={12} />
            <span>{branchLabel}</span>
          </span>
        {/if}
        <span class="rules" class:rules-empty={subscription.ruleCount === 0}>{ruleCountText}</span>
        {#if suggestionsPending > 0 && onsuggestions}
          <button
            type="button"
            class="suggestions-chip"
            title={suggestionsLabel}
            aria-label={suggestionsLabel}
            onclick={onsuggestions}
          >
            <Lightbulb size={12} />
            <span class="suggestions-count">{suggestionsPending}</span>
          </button>
        {/if}
      </div>
    </div>
  </div>

  {#if incomingFiles.length > 0 && incomingOpen}
    <IncomingChangesList id={incomingId} files={incomingFiles} {onviewdiff} />
  {/if}

  <div class="dates-row">
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
    <div class="zone-expand">
      {#if incomingFiles.length > 0}
        <ExpandToggle
          expanded={incomingOpen}
          label={incomingLabel}
          count={incomingFiles.length}
          controls={incomingId}
          attention="down"
          onclick={() => (incomingOpen = !incomingOpen)}
        />
      {/if}
    </div>
  </div>

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
    <Tooltip text={t("subscriptions.card.update_now")}>
      <Button
        size="sm"
        variant={behind > 0 ? "primary" : "secondary"}
        loading={updating}
        disabled={busy}
        ariaLabel={t("subscriptions.card.update_now")}
        onclick={onupdate}
      >
        <CloudDownload size={14} />
        <span>{t("subscriptions.card.update_short")}</span>
      </Button>
    </Tooltip>
    <Tooltip text={t("subscriptions.card.check")}>
      <Button
        size="sm"
        variant="secondary"
        loading={checking}
        disabled={busy}
        ariaLabel={t("subscriptions.card.check")}
        onclick={oncheck}
      >
        {#if !checking}
          <RefreshCw size={14} />
        {/if}
        <span>{t("subscriptions.card.check_short")}</span>
      </Button>
    </Tooltip>
    <Tooltip text={t("subscriptions.card.browse")}>
      <Button
        size="sm"
        variant="secondary"
        ariaLabel={t("subscriptions.card.browse")}
        onclick={onbrowse}
      >
        <FolderTree size={14} />
        <span>{t("subscriptions.card.browse_short")}</span>
      </Button>
    </Tooltip>
    <span class="actions-end">
      <IconButton
        label={t("subscriptions.card.remove_for", { name: subscription.name })}
        disabled={busy}
        onclick={onremove}
      >
        <Trash2 size={16} />
      </IconButton>
    </span>
  </div>
</article>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5);
    background-color: var(--surface);
    border-radius: var(--radius-lg);
    box-shadow: var(--elev-1);
    min-width: 0;
    transition: box-shadow var(--duration-fast) var(--easing-out);
  }

  .card:hover {
    box-shadow: var(--elev-2);
  }

  /* Fixed zones: tile, text column, expand control. The head has the same height on every card. */
  .card-top {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    align-items: start;
    gap: var(--space-3);
    min-width: 0;
  }

  .zone-expand {
    display: flex;
    justify-content: flex-end;
    min-width: var(--card-zone-expand);
    min-height: var(--card-zone-icon);
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
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .title-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
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

  .meta {
    display: flex;
    align-items: center;
    flex-wrap: nowrap;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
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
    max-width: 60%;
  }

  .branch-link {
    padding: 0;
    border: none;
    background: none;
    font-size: inherit;
    color: inherit;
    cursor: pointer;
    border-radius: var(--radius-sm);
  }

  .branch-link:hover span {
    color: var(--ink);
    text-decoration: underline;
  }

  .branch-link:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
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

  .dates-row {
    display: grid;
    grid-template-columns: repeat(2, max-content) minmax(var(--card-zone-expand), 1fr);
    align-items: center;
    gap: var(--space-4);
  }

  .dates {
    display: contents;
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
    gap: 2px;
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
    gap: var(--space-2);
  }

  /* Sits at the end of the meta line so the action row keeps its width. */
  .suggestions-chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    flex-shrink: 0;
    margin: -3px 0 -3px auto;
    padding: 3px var(--space-2);
    border: none;
    border-radius: var(--radius-sm);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--ink);
    font-size: var(--text-xs);
    line-height: 1.2;
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--easing-out);
  }

  .suggestions-chip:hover {
    background-color: var(--surface);
  }

  .suggestions-chip:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .suggestions-count {
    font-variant-numeric: tabular-nums;
  }

  .actions-end {
    margin-left: auto;
    display: inline-flex;
  }
</style>
