<script lang="ts">
  import {
    BookOpen,
    Copy,
    ArrowRight,
    ExternalLink,
    FolderOpen,
    GitBranch,
    HardDrive,
    MoreHorizontal,
    Pencil,
    Rss,
    Trash2,
  } from "@lucide/svelte";
  import type { rulesync } from "../../../../wailsjs/go/models";
  import { t } from "../../i18n";
  import { appState } from "../../stores/appState.svelte";
  import ExpandToggle from "../ExpandToggle.svelte";
  import IconButton from "../IconButton.svelte";
  import Switch from "../Switch.svelte";
  import Badge from "./Badge.svelte";
  import DestinationStatusRow from "./DestinationStatusRow.svelte";
  import StatusDot from "./StatusDot.svelte";
  import "../cardZones.css";
  import {
    AUTO_KIND,
    baseName,
    destinationLabel,
    destinationSummary,
    isFolderKind,
    needsReview,
    singleDestinationFile,
    type DotTone,
  } from "./ruleUi";

  /** Selections of more than one file behave like a folder: their files are listed on demand. */
  const MULTI_SELECTION_MIN = 2;
  /** Tones ordered from most to least relevant for a rule-level summary. */
  const TONE_ORDER: readonly DotTone[] = ["conflict", "up", "down", "idle"];
  const SUMMARY_SEPARATOR = " · ";

  interface Props {
    status: rulesync.RuleStatus;
    /** Tighter padding; the row layout itself adapts to the width of the card. */
    compact?: boolean;
    /** Absolute source path when it can be resolved on this PC ("" otherwise). */
    sourcePath: string;
    onsource?: () => void;
    ontoggle: (enabled: boolean) => void;
    onedit: () => void;
    onduplicate: () => void;
    ondelete: () => void;
    onopen: (path: string) => void;
    /** Opens the read-only source viewer, optionally with a source-relative file selected. */
    onview: (path?: string) => void;
    onreview: (destId: string, file: rulesync.FileStatus) => void;
  }

  let {
    status,
    compact = false,
    sourcePath,
    onsource,
    ontoggle,
    onedit,
    onduplicate,
    ondelete,
    onopen,
    onview,
    onreview,
  }: Props = $props();

  const uid = $props.id();
  const detailsId = `rule-details-${uid}`;

  let menuOpen = $state(false);
  let expanded = $state(false);
  let menuContainer = $state<HTMLDivElement | null>(null);

  const rule = $derived(status.rule);
  const subscriptionName = $derived(
    appState.config?.subscriptions?.find(
      (subscription) => subscription.id === rule.source.subscription
    )?.name || rule.source.subscription
  );
  const scope = $derived(status.scope === "local" ? "local" : "repo");
  const kind = $derived(status.detectedKind || rule.kind);
  const forcedKind = $derived(Boolean(rule.kind && rule.kind !== AUTO_KIND));
  const commonMode = $derived(
    rule.destinations.length > 0 &&
      rule.destinations.every((d) => d.mode === rule.destinations[0].mode)
      ? rule.destinations[0].mode
      : ""
  );
  const destinationChips = $derived(
    rule.destinations.map((destination) => ({
      destination,
      summary: destinationSummary(
        rule,
        destination,
        status.destinations.find((s) => s.destId === destination.id)
      ),
    }))
  );
  function mostRelevantTone(tones: DotTone[]): DotTone | undefined {
    return TONE_ORDER.find((tone) => tones.includes(tone));
  }
  const overallEntry = $derived.by(() => {
    const tone = mostRelevantTone(destinationChips.map((entry) => entry.summary.tone));
    return destinationChips.find((entry) => entry.summary.tone === tone) ?? destinationChips[0];
  });

  /** The row shows the first destination; the others (and folder contents) live in the details. */
  const primary = $derived(rule.destinations[0]);
  const primaryStatus = $derived(status.destinations.find((s) => s.destId === primary?.id));
  const primaryFile = $derived(primary ? singleDestinationFile(primary, primaryStatus) : null);
  const primaryReady = $derived((primaryStatus?.state ?? "ready") === "ready");
  /** Review replaces the mode badge: it is the only per-row state that needs the user's attention. */
  const primaryReview = $derived(
    primary && primaryFile && primaryReady && needsReview(primaryFile.status, rule, primary)
      ? primaryFile
      : null
  );

  /** Derived from the rule itself so the toggle does not appear or vanish when a rule is disabled. */
  const hasFileList = $derived(
    isFolderKind(kind) || (rule.source.include?.length ?? 0) >= MULTI_SELECTION_MIN
  );
  const fileCount = $derived(
    Math.max(
      rule.source.include?.length ?? 0,
      ...status.destinations.map((destination) => destination.files?.length ?? 0)
    )
  );
  const destinationCount = $derived(rule.destinations.length);
  const expandable = $derived(hasFileList || destinationCount > 1);
  const expandCount = $derived(
    hasFileList ? (fileCount > 0 ? fileCount : undefined) : destinationCount
  );
  const expandSummary = $derived(
    [
      hasFileList && fileCount > 0
        ? t(`rules.card.summary_files_${fileCount === 1 ? "one" : "other"}`, { count: fileCount })
        : "",
      destinationCount > 1
        ? t(`rules.card.summary_destinations_${destinationCount === 1 ? "one" : "other"}`, {
            count: destinationCount,
          })
        : "",
    ]
      .filter(Boolean)
      .join(SUMMARY_SEPARATOR)
  );
  const expandLabel = $derived(
    t(expanded ? "rules.card.hide_details" : "rules.card.show_details", { name: rule.name })
  );
  /** Problems that are not visible on the collapsed row. */
  const hiddenAttention = $derived(
    mostRelevantTone(
      (hasFileList ? destinationChips : destinationChips.slice(1)).map(
        (entry) => entry.summary.tone
      )
    )
  );

  const openableDestinations = $derived(
    rule.destinations
      .map((d) => ({
        dest: d,
        root: status.destinations.find((s) => s.destId === d.id)?.targetRoot ?? "",
      }))
      .filter((entry) => entry.root)
  );

  const ORIGIN_ICONS = { repo: GitBranch, local: HardDrive, subscription: Rss } as const;
  const OriginIcon = $derived(
    ORIGIN_ICONS[rule.source.origin as keyof typeof ORIGIN_ICONS] ?? GitBranch
  );
  const originLabel = $derived(t(`rules.origin.${rule.source.origin}`));
  /** The icon carries what the removed badge used to say: where the source is and where the rule lives. */
  const originTitle = $derived(
    [
      originLabel,
      ...(rule.source.origin === "subscription" ? [subscriptionName] : []),
      t(
        rule.source.origin === "subscription"
          ? "rules.scope.subscription_hint"
          : `rules.scope.${scope}_hint`
      ),
    ].join("\n")
  );
  const sourceName = $derived(
    rule.source.include?.length
      ? t(`rules.editor.selection_name_${rule.source.include.length === 1 ? "one" : "other"}`, {
          count: rule.source.include.length,
        })
      : baseName(rule.source.path) || rule.source.path
  );
  const sourceTitle = $derived(
    [
      originLabel,
      ...(rule.source.origin === "subscription" ? [subscriptionName] : []),
      sourcePath || rule.source.path,
      ...(rule.source.include ?? []),
      ...(kind
        ? [
            `${t(forcedKind ? "rules.card.kind_forced" : "rules.card.kind_detected")}: ${t(`rules.kind.${kind}`)}`,
          ]
        : []),
    ].join("\n")
  );
  const primaryTitle = $derived(
    primary
      ? [
          destinationLabel(primary),
          t(`rules.mode.${primary.mode}`),
          ...(primary.transform
            ? [`${t(`rules.transform.${primary.transform.type}`)} · ${primary.transform.key}`]
            : []),
        ].join("\n")
      : ""
  );

  function closeMenu() {
    menuOpen = false;
  }

  function handleWindowPointerDown(e: PointerEvent) {
    if (!menuOpen || !menuContainer) return;
    if (menuContainer.contains(e.target as Node)) return;
    closeMenu();
  }

  function handleWindowKeyDown(e: KeyboardEvent) {
    if (menuOpen && e.key === "Escape") closeMenu();
  }
</script>

<svelte:window onpointerdown={handleWindowPointerDown} onkeydown={handleWindowKeyDown} />

<article
  class="rule-card"
  class:compact
  class:rule-disabled={!rule.enabled}
  class:menu-open={menuOpen}
  aria-label={rule.name}
>
  <div class="rule-row">
    <div class="zone zone-name">
      {#if overallEntry}
        <span
          class="rule-state"
          role="img"
          aria-label={t("rules.card.rule_status", { status: t(overallEntry.summary.labelKey) })}
          title={t("rules.card.rule_status", { status: t(overallEntry.summary.labelKey) })}
        >
          <StatusDot tone={overallEntry.summary.tone} />
        </span>
      {/if}
      <h3 class="rule-name" title={rule.name}>{rule.name}</h3>
    </div>

    <div class="zone zone-source">
      <span class="origin-icon" aria-label={originLabel} title={originTitle} role="img">
        <OriginIcon size={14} />
      </span>
      <button type="button" class="source-name" title={sourceTitle} onclick={onsource}
        >{sourceName}</button
      >
    </div>

    <div class="zone zone-expand">
      {#if expandable}
        <ExpandToggle
          {expanded}
          label={expandLabel}
          title={expandSummary || expandLabel}
          count={expandCount}
          controls={detailsId}
          attention={hiddenAttention}
          onclick={() => (expanded = !expanded)}
        />
      {/if}
    </div>

    <div class="zone zone-chip">
      {#if rule.source.origin === "subscription" && rule.source.subscription}
        <Badge truncate title={rule.source.subscription}>{subscriptionName}</Badge>
      {/if}
    </div>

    <div class="zone zone-arrow">
      {#if primary}<ArrowRight size={12} aria-hidden="true" />{/if}
    </div>

    <div class="zone zone-dest-chip">
      {#if primary}
        <Badge tone="accent" mono truncate title={primaryTitle}>{primary.location}</Badge>
      {/if}
    </div>

    <div class="zone zone-dest-file">
      {#if primary}<span class="dest-path" title={primaryTitle}>{primary.path}</span>{/if}
    </div>

    <div class="zone zone-mode">
      {#if primary && primaryReview}
        <button
          type="button"
          class="review-btn"
          aria-label={t("rules.review.button_aria", { file: primaryReview.relPath })}
          onclick={() => onreview(primary.id, primaryReview)}
        >
          {t("rules.review.button")}
        </button>
      {:else if primary && primary.transform}
        <span class="mode-badge">
          <Badge tone="info" truncate title={primaryTitle}>
            {t(`rules.transform.${primary.transform.type}`)}
          </Badge>
        </span>
      {:else if primary && commonMode}
        <span class="mode-badge"><Badge truncate>{t(`rules.mode.${commonMode}`)}</Badge></span>
      {/if}
    </div>

    <div class="zone zone-view">
      <IconButton
        label={t("rules.card.view_source_named", { name: rule.name })}
        onclick={() => onview()}
      >
        <BookOpen size={14} />
      </IconButton>
    </div>

    <div class="zone zone-open">
      {#if primaryStatus?.targetRoot}
        <IconButton
          label={t("rules.card.open_destination_named", { location: primary.location })}
          onclick={() => onopen(primaryStatus?.targetRoot ?? "")}
        >
          <FolderOpen size={14} />
        </IconButton>
      {/if}
    </div>

    <div class="zone zone-toggle">
      <Switch
        checked={rule.enabled}
        ariaLabel={t("rules.card.enabled")}
        onchange={(next) => ontoggle(next)}
      />
    </div>

    <div class="zone zone-menu">
      <div class="menu-container" bind:this={menuContainer}>
        <IconButton
          label={t("rules.card.menu_toggle", { name: rule.name })}
          onclick={() => (menuOpen = !menuOpen)}
          ariaExpanded={menuOpen}
        >
          <MoreHorizontal size={16} />
        </IconButton>

        {#if menuOpen}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <div class="dropdown-menu" onclick={closeMenu} role="menu" tabindex="-1">
            <button type="button" class="dropdown-item" role="menuitem" onclick={onedit}>
              <Pencil size={14} />
              <span>{t("rules.card.edit")}</span>
            </button>
            <button type="button" class="dropdown-item" role="menuitem" onclick={onduplicate}>
              <Copy size={14} />
              <span>{t("rules.card.duplicate")}</span>
            </button>
            {#if sourcePath}
              <button
                type="button"
                class="dropdown-item"
                role="menuitem"
                onclick={() => onopen(sourcePath)}
              >
                <ExternalLink size={14} />
                <span>{t("rules.card.open_source")}</span>
              </button>
            {/if}
            {#each openableDestinations as entry (entry.dest.id)}
              <button
                type="button"
                class="dropdown-item"
                role="menuitem"
                onclick={() => onopen(entry.root)}
              >
                <ExternalLink size={14} />
                <span class="item-text">
                  {t("rules.card.open_destination_named", {
                    location: destinationLabel(entry.dest),
                  })}
                </span>
              </button>
            {/each}
            <div class="menu-sep" role="separator"></div>
            <button type="button" class="dropdown-item danger" role="menuitem" onclick={ondelete}>
              <Trash2 size={14} />
              <span>{t("rules.card.delete")}</span>
            </button>
          </div>
        {/if}
      </div>
    </div>
  </div>

  {#if expandable && expanded}
    <div
      class="details"
      id={detailsId}
      role="group"
      aria-label={t("rules.card.destinations_label", { count: destinationCount })}
    >
      {#each rule.destinations as destination (destination.id)}
        <DestinationStatusRow
          {rule}
          {destination}
          status={status.destinations.find((s) => s.destId === destination.id)}
          {onreview}
          {onopen}
          {onview}
        />
      {/each}
    </div>
  {/if}
</article>

<style>
  .rule-card {
    /* The row layout reacts to the width of the card, not of the window. */
    container-type: inline-size;
    background-color: var(--surface);
    border-radius: var(--radius-lg);
    box-shadow: var(--elev-1);
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    min-width: 0;
    transition: box-shadow var(--duration-fast) var(--easing-out);
  }

  .rule-card.compact {
    padding: var(--space-3) var(--space-4);
  }

  .rule-card:hover {
    box-shadow: var(--elev-2);
  }

  /* Containment creates a stacking context: lift the card while its menu is open. */
  .rule-card.menu-open {
    position: relative;
    z-index: 20;
  }

  /*
   * Every card uses the same tracks: fixed zones plus fractions that do not depend on content,
   * so each indicator lines up from one card to the next. Empty zones stay reserved.
   */
  .rule-row {
    display: grid;
    align-items: center;
    column-gap: var(--card-zone-gap);
    min-height: var(--card-zone-icon);
    grid-template-columns:
      var(--card-track-name)
      var(--card-track-source)
      var(--card-zone-expand)
      var(--card-zone-chip)
      var(--card-zone-arrow)
      var(--card-zone-dest-chip)
      var(--card-track-file)
      var(--card-zone-mode)
      var(--card-zone-icon)
      var(--card-zone-icon)
      var(--card-zone-toggle)
      var(--card-zone-icon);
    grid-template-areas: "name source expand chip arrow destchip destfile mode view open toggle menu";
  }

  .zone {
    display: flex;
    align-items: center;
    min-width: 0;
    gap: var(--space-2);
  }

  .zone-name {
    grid-area: name;
  }
  .zone-source {
    grid-area: source;
    color: var(--text-muted);
  }
  .zone-expand {
    grid-area: expand;
    justify-content: center;
  }
  .zone-chip {
    grid-area: chip;
  }
  .zone-arrow {
    grid-area: arrow;
    justify-content: center;
    color: var(--text-subtle);
  }
  .zone-dest-chip {
    grid-area: destchip;
  }
  .zone-dest-file {
    grid-area: destfile;
  }
  .zone-mode {
    grid-area: mode;
  }
  .zone-view {
    grid-area: view;
    justify-content: center;
  }
  .zone-open {
    grid-area: open;
    justify-content: center;
  }
  .zone-toggle {
    grid-area: toggle;
  }
  .zone-menu {
    grid-area: menu;
    justify-content: center;
  }

  /* Transparent wrapper: lets the compact tiers hide the badge without touching the Review button. */
  .mode-badge {
    display: contents;
  }

  /*
   * Narrower cards drop the secondary zones (repository chip, mode badge) for every card alike.
   * The mode zone shrinks to the Review button, which still needs a place.
   */
  @container (max-width: 959px) {
    .rule-row {
      grid-template-columns:
        var(--card-track-name)
        var(--card-track-source)
        var(--card-zone-expand)
        var(--card-zone-arrow)
        var(--card-zone-dest-chip)
        var(--card-track-file)
        var(--card-zone-review)
        var(--card-zone-icon)
        var(--card-zone-icon)
        var(--card-zone-toggle)
        var(--card-zone-icon);
      grid-template-areas: "name source expand arrow destchip destfile mode view open toggle menu";
    }

    .zone-chip,
    .mode-badge {
      display: none;
    }
  }

  /* Narrow cards (grid view, small windows) stack the same zones on three fixed rows. */
  @container (max-width: 759px) {
    .rule-row {
      row-gap: var(--space-2);
      grid-template-columns:
        minmax(0, 1fr)
        var(--card-zone-expand)
        var(--card-zone-icon)
        var(--card-zone-icon)
        var(--card-zone-toggle)
        var(--card-zone-icon);
      grid-template-areas:
        "name expand view open toggle menu"
        "source source source source source arrow"
        "destchip destfile destfile destfile mode mode";
    }

    .zone-arrow :global(svg) {
      transform: rotate(90deg);
    }
  }

  .rule-state {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    width: var(--card-zone-dot);
  }

  .rule-name {
    margin: 0;
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
    letter-spacing: -0.01em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }

  .origin-icon {
    display: inline-flex;
    flex-shrink: 0;
  }

  .source-name {
    min-width: 0;
    padding: 0;
    border: 0;
    background: none;
    cursor: pointer;
    text-align: left;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .source-name:hover {
    text-decoration: underline;
  }

  .source-name:focus-visible {
    outline: 2px solid var(--accent);
  }

  .dest-path {
    min-width: 0;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
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

  /* A disabled rule is only dimmed: nothing is hidden, swapped or resized. */
  .rule-disabled .zone:not(.zone-toggle, .zone-menu) {
    opacity: var(--card-disabled-opacity);
  }

  .details {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding-top: var(--space-2);
    border-top: 1px solid var(--border-subtle);
  }

  .menu-container {
    position: relative;
  }

  .dropdown-menu {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: var(--space-1);
    background-color: var(--surface);
    border-radius: var(--radius-md);
    box-shadow: var(--elev-3);
    min-width: 200px;
    max-width: 320px;
    padding: var(--space-1);
    z-index: 50;
    display: flex;
    flex-direction: column;
  }

  .dropdown-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    font-size: var(--text-xs);
    color: var(--ink);
    background: none;
    border: none;
    border-radius: var(--radius-xs);
    cursor: pointer;
    text-align: left;
    transition: background-color var(--duration-fast) var(--easing-out);
  }

  .dropdown-item:hover,
  .dropdown-item:focus-visible {
    outline: none;
    background-color: var(--surface-sunken);
  }

  .dropdown-item.danger {
    color: var(--dot-conflict);
  }

  .item-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .menu-sep {
    height: 1px;
    margin: var(--space-1) 0;
    background-color: var(--hairline);
  }
</style>
