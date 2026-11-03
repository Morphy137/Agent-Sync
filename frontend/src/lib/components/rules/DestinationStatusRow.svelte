<script lang="ts">
  import { ArrowRight, FolderOpen } from "@lucide/svelte";
  import type { rules, rulesync } from "../../../../wailsjs/go/models";
  import { t } from "../../i18n";
  import IconButton from "../IconButton.svelte";
  import Badge from "./Badge.svelte";
  import StatusDot from "./StatusDot.svelte";
  import "../cardZones.css";
  import {
    SOURCE_DELETED,
    destinationSummary,
    fileDotTone,
    needsReview,
    singleDestinationFile,
  } from "./ruleUi";

  /** One destination inside the expanded part of a rule card, with the files it syncs. */
  interface Props {
    rule: rules.Rule;
    destination: rules.Destination;
    status?: rulesync.DestinationStatus;
    onreview: (destId: string, file: rulesync.FileStatus) => void;
    onopen: (path: string) => void;
    /** Opens the read-only source viewer with this source-relative file selected. */
    onview: (path: string) => void;
  }

  let { rule, destination, status, onreview, onopen, onview }: Props = $props();

  const destState = $derived(status?.state ?? (destination.enabled ? "ready" : "disabled"));
  const files = $derived(status?.files ?? []);
  /** A file destination has one file: its status is shown inline instead of as a list. */
  const singleFile = $derived(singleDestinationFile(destination, status));
  const summary = $derived(destinationSummary(rule, destination, status));
  const statusLabel = $derived(
    singleFile && destState === "ready"
      ? t(`rules.file_status.${singleFile.status}`)
      : t(`rules.dest_state.${destState}`)
  );
  const destinationTitle = $derived(
    [
      `${destination.location}/${destination.path}`,
      t(`rules.mode.${destination.mode}`),
      t(summary.labelKey),
    ].join("\n")
  );
</script>

<div class="dest" class:dest-muted={destState === "disabled"}>
  <div class="dest-head">
    <span class="dest-dot" role="img" aria-label={t(summary.labelKey)} title={t(summary.labelKey)}>
      <StatusDot tone={summary.tone} />
    </span>
    <div class="dest-target" title={destinationTitle}>
      <Badge tone="accent" mono truncate>{destination.location}</Badge>
      <ArrowRight size={12} aria-hidden="true" />
      <span class="dest-path">{destination.path}</span>
    </div>
    <div class="dest-mode">
      {#if singleFile && destState === "ready" && needsReview(singleFile.status, rule, destination)}
        <button
          type="button"
          class="review-btn"
          aria-label={t("rules.review.button_aria", { file: singleFile.relPath })}
          onclick={() => onreview(destination.id, singleFile)}
        >
          {t("rules.review.button")}
        </button>
      {:else}
        <span class="mode-badge"><Badge truncate>{t(`rules.mode.${destination.mode}`)}</Badge></span
        >
      {/if}
    </div>
    <span class="dest-state" title={statusLabel}>{statusLabel}</span>
    <div class="dest-open">
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

  {#if destination.transform}
    <p class="dest-note">
      {t(`rules.transform.${destination.transform.type}`)} · {destination.transform.key}
    </p>
  {/if}

  {#if !singleFile}
    {#if files.length > 0}
      <ul class="file-list" aria-label={t("rules.card.files_label")}>
        {#each files as file (file.relPath)}
          <li class="file-row">
            <span class="file-dot"><StatusDot tone={fileDotTone(file.status)} /></span>
            {#if file.status === SOURCE_DELETED}
              <span class="file-path" title={file.targetAbs}>{file.relPath}</span>
            {:else}
              <button
                type="button"
                class="file-path file-link"
                title={file.targetAbs}
                aria-label={t("rules.viewer.view_file", { file: file.relPath })}
                onclick={() => onview(file.relPath)}>{file.relPath}</button
              >
            {/if}
            <div class="file-review">
              {#if needsReview(file.status, rule, destination)}
                <button
                  type="button"
                  class="review-btn"
                  aria-label={t("rules.review.button_aria", { file: file.relPath })}
                  onclick={() => onreview(destination.id, file)}
                >
                  {t("rules.review.button")}
                </button>
              {/if}
            </div>
            <span class="file-status">{t(`rules.file_status.${file.status}`)}</span>
          </li>
        {/each}
      </ul>
    {:else if destState === "ready"}
      <p class="dest-note">{t("rules.card.no_files")}</p>
    {/if}
  {/if}
</div>

<style>
  /* Both the destination line and its file rows use these tracks, so their columns line up. */
  .dest-head,
  .file-row {
    display: grid;
    align-items: center;
    column-gap: var(--card-zone-gap);
    min-height: 28px;
    grid-template-columns:
      var(--card-zone-dot)
      minmax(0, 1fr)
      var(--card-zone-mode)
      var(--card-zone-status)
      var(--card-zone-icon);
    font-size: var(--text-xs);
  }

  .dest {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .dest-muted {
    opacity: var(--card-disabled-opacity);
  }

  .dest-dot {
    display: inline-flex;
    justify-content: center;
  }

  .dest-target {
    display: grid;
    align-items: center;
    column-gap: var(--card-zone-gap);
    grid-template-columns: var(--card-zone-dest-chip) var(--card-zone-arrow) minmax(0, 1fr);
    min-width: 0;
    color: var(--text-subtle);
  }

  .dest-path {
    min-width: 0;
    font-family: var(--font-mono);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .dest-mode,
  .dest-open,
  .file-review {
    display: flex;
    align-items: center;
    min-width: 0;
  }

  .dest-open {
    justify-content: center;
  }

  .dest-state,
  .file-status {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .dest-note {
    margin: 0;
    padding-left: calc(var(--card-zone-dot) + var(--card-zone-gap));
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

  .file-dot {
    display: inline-flex;
    justify-content: center;
  }

  .file-path {
    min-width: 0;
    font-family: var(--font-mono);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-link {
    justify-self: start;
    max-width: 100%;
    padding: 0;
    border: 0;
    background: none;
    font-size: inherit;
    text-align: left;
    cursor: pointer;
  }

  .file-link:hover {
    text-decoration: underline;
  }

  .file-link:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
    border-radius: var(--radius-xs);
  }

  .dest-state,
  .file-status {
    color: var(--text-muted);
  }

  .review-btn {
    height: 24px;
    max-width: 100%;
    padding: 0 var(--space-2);
    border: none;
    border-radius: var(--radius-xs);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    color: var(--ink);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    cursor: pointer;
  }

  .review-btn:hover {
    box-shadow: var(--elev-2);
  }

  .review-btn:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  /* Transparent wrapper: lets narrow cards hide the badge without touching the Review button. */
  .mode-badge {
    display: contents;
  }

  /* Narrow cards: keep the target, Review and actions; the mode is also shown on the row tooltip. */
  @container (max-width: 719px) {
    .dest-head,
    .file-row {
      grid-template-columns:
        var(--card-zone-dot)
        minmax(0, 1fr)
        var(--card-zone-review)
        var(--card-zone-icon);
    }

    .dest-state,
    .file-status {
      display: none;
    }

    .mode-badge {
      display: none;
    }
  }
</style>
