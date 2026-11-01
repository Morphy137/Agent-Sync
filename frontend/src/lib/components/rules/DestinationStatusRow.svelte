<script lang="ts">
  import { ArrowRight, ChevronDown, ChevronRight, FolderOpen } from "@lucide/svelte";
  import type { rules, rulesync } from "../../../../wailsjs/go/models";
  import { t } from "../../i18n";
  import IconButton from "../IconButton.svelte";
  import Badge from "./Badge.svelte";
  import StatusDot from "./StatusDot.svelte";
  import { IN_SYNC, destStateTone, destinationSummary, fileDotTone, needsReview } from "./ruleUi";

  interface Props {
    rule: rules.Rule;
    destination: rules.Destination;
    status?: rulesync.DestinationStatus;
    compact?: boolean;
    plain?: boolean;
    hideMode?: boolean;
    sourceName?: string;
    sourceTitle?: string;
    sourceSubscription?: string;
    onreview: (file: rulesync.FileStatus) => void;
    onopen: (path: string) => void;
  }

  let {
    rule,
    destination,
    status,
    compact = false,
    plain = false,
    hideMode = false,
    sourceName = "",
    sourceTitle = "",
    sourceSubscription,
    onreview,
    onopen,
  }: Props = $props();

  let showAll = $state(false);

  const destState = $derived(status?.state ?? (destination.enabled ? "ready" : "disabled"));
  const files = $derived(status?.files ?? []);
  const pendingFiles = $derived(files.filter((f) => f.status !== IN_SYNC));
  const inSyncCount = $derived(files.length - pendingFiles.length);
  const visibleFiles = $derived(showAll ? files : pendingFiles);
  /** A file destination has one file: its status is shown inline instead of as a list. */
  const singleFile = $derived(
    files.length === 1 && files[0].relPath === destination.path.split("/").pop() ? files[0] : null
  );
  const summary = $derived(destinationSummary(rule, destination, status));
</script>

<div class="dest" class:compact class:plain class:dest-muted={destState === "disabled"}>
  <div class="dest-head">
    <div class="dest-target">
      {#if compact}
        <span
          class="compact-state"
          role="img"
          aria-label={t(summary.labelKey)}
          title={t(summary.labelKey)}
        >
          <StatusDot tone={summary.tone} />
        </span>
        <span class="source-name" title={sourceTitle}>{sourceName}</span>
        {#if sourceSubscription}<span class="compact-label"
            ><Badge mono title={sourceSubscription}>{sourceSubscription}</Badge></span
          >{/if}
        <ArrowRight size={12} aria-hidden="true" />
      {/if}
      <span class:compact-label={compact} title={destination.location}
        ><Badge tone="accent" mono>{destination.location}</Badge></span
      >
      {#if !compact}<ArrowRight size={12} aria-hidden="true" />{/if}
      <span class="dest-path" title={destination.path}>{destination.path}</span>
    </div>
    <div class="dest-meta">
      {#if !hideMode}<Badge>{t(`rules.mode.${destination.mode}`)}</Badge>{/if}
      {#if destination.transform}
        <Badge
          tone="info"
          title={`${t(`rules.transform.${destination.transform.type}`)} · ${destination.transform.key}`}
        >
          {t(`rules.transform.${destination.transform.type}`)} · {destination.transform.key}
        </Badge>
      {/if}
      {#if compact}
        {#if singleFile && destState === "ready" && needsReview(singleFile.status, rule, destination)}
          <button
            type="button"
            class="review-btn"
            aria-label={t("rules.review.button_aria", { file: singleFile.relPath })}
            onclick={() => onreview(singleFile)}
          >
            {t("rules.review.button")}
          </button>
        {/if}
      {:else if singleFile && destState === "ready"}
        <span class="dest-state">
          <StatusDot tone={fileDotTone(singleFile.status)} />
          <span>{t(`rules.file_status.${singleFile.status}`)}</span>
        </span>
        {#if needsReview(singleFile.status, rule, destination)}
          <button
            type="button"
            class="review-btn"
            aria-label={t("rules.review.button_aria", { file: singleFile.relPath })}
            onclick={() => onreview(singleFile)}
          >
            {t("rules.review.button")}
          </button>
        {/if}
      {:else}
        <span class="dest-state">
          <StatusDot tone={destStateTone(destState)} />
          <span>{t(`rules.dest_state.${destState}`)}</span>
        </span>
      {/if}
      {#if status?.targetRoot}
        <IconButton
          label={t("rules.card.open_destination_named", { location: destination.location })}
          onclick={() => onopen(status?.targetRoot ?? "")}
        >
          <FolderOpen size={14} />
        </IconButton>
      {/if}
    </div>
  </div>

  {#if !singleFile && visibleFiles.length > 0}
    <ul class="file-list" aria-label={t("rules.card.files_label")}>
      {#each visibleFiles as file (file.relPath)}
        <li class="file-row">
          <div class="file-left">
            <StatusDot tone={fileDotTone(file.status)} />
            <span class="file-path" title={file.targetAbs}>{file.relPath}</span>
          </div>
          <div class="file-right">
            <span class="file-status">{t(`rules.file_status.${file.status}`)}</span>
            {#if needsReview(file.status, rule, destination)}
              <button
                type="button"
                class="review-btn"
                aria-label={t("rules.review.button_aria", { file: file.relPath })}
                onclick={() => onreview(file)}
              >
                {t("rules.review.button")}
              </button>
            {/if}
          </div>
        </li>
      {/each}
    </ul>
  {/if}

  {#if !singleFile && inSyncCount > 0}
    <button
      type="button"
      class="toggle-files"
      aria-expanded={showAll}
      onclick={() => (showAll = !showAll)}
    >
      {#if showAll}
        <ChevronDown size={14} />
        <span>{t("rules.card.hide_in_sync")}</span>
      {:else}
        <ChevronRight size={14} />
        <span>{t("rules.card.show_in_sync", { count: inSyncCount })}</span>
      {/if}
    </button>
  {/if}
</div>

<style>
  .dest {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .dest-muted {
    opacity: 0.65;
  }

  .dest.compact {
    padding: 0;
    background: transparent;
    box-shadow: none;
  }

  .dest.plain {
    padding: var(--space-1) 0;
    border-radius: 0;
    border-bottom: 1px solid var(--border-subtle);
    background: transparent;
    box-shadow: none;
  }

  .dest.plain:last-child {
    border-bottom: 0;
  }

  .compact .dest-head {
    flex-wrap: nowrap;
  }

  .compact .dest-target {
    flex: 1 1 0;
  }

  .compact .dest-meta {
    justify-content: flex-end;
    min-width: 0;
    max-width: 50%;
  }

  .compact-label {
    flex: 0 0 auto;
    min-width: 0;
    max-width: 25%;
    overflow: hidden;
  }

  .compact-state {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .source-name {
    flex: 0 0 auto;
    max-width: 35%;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--ink);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }

  .dest-target :global(svg) {
    flex-shrink: 0;
  }

  .dest-head {
    display: flex;
    min-height: var(--space-8);
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .dest-target {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    flex: 1 1 200px;
    color: var(--text-subtle);
  }

  .dest-path {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .compact .dest-path {
    flex: 0 1 auto;
  }

  .dest-meta {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .dest-state {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .file-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
  }

  .file-row {
    min-height: 28px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    font-size: var(--text-xs);
  }

  .file-left,
  .file-right {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }

  .file-right {
    flex-shrink: 0;
  }

  .file-path {
    font-family: var(--font-mono);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-status {
    color: var(--text-muted);
  }

  .review-btn {
    height: 24px;
    padding: 0 var(--space-2);
    border: none;
    border-radius: var(--radius-xs);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    color: var(--ink);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    cursor: pointer;
  }

  .review-btn:hover {
    box-shadow: var(--elev-2);
  }

  .review-btn:focus-visible,
  .toggle-files:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .toggle-files {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    align-self: flex-start;
    background: none;
    border: none;
    border-radius: var(--radius-xs);
    padding: 0;
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    cursor: pointer;
  }

  .toggle-files:hover {
    color: var(--ink);
  }
</style>
