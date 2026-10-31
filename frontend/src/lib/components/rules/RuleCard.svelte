<script lang="ts">
  import {
    Copy,
    ExternalLink,
    GitBranch,
    HardDrive,
    MoreHorizontal,
    Pencil,
    Rss,
    Trash2,
  } from "@lucide/svelte";
  import type { rulesync } from "../../../../wailsjs/go/models";
  import { t } from "../../i18n";
  import IconButton from "../IconButton.svelte";
  import Switch from "../Switch.svelte";
  import Badge from "./Badge.svelte";
  import DestinationStatusRow from "./DestinationStatusRow.svelte";
  import { AUTO_KIND, baseName, destinationLabel } from "./ruleUi";

  interface Props {
    status: rulesync.RuleStatus;
    /** Absolute source path when it can be resolved on this PC ("" otherwise). */
    sourcePath: string;
    ontoggle: (enabled: boolean) => void;
    onedit: () => void;
    onduplicate: () => void;
    ondelete: () => void;
    onopen: (path: string) => void;
    onreview: (destId: string, file: rulesync.FileStatus) => void;
  }

  let { status, sourcePath, ontoggle, onedit, onduplicate, ondelete, onopen, onreview }: Props =
    $props();

  let menuOpen = $state(false);
  let menuContainer = $state<HTMLDivElement | null>(null);

  const rule = $derived(status.rule);
  const scope = $derived(status.scope === "local" ? "local" : "repo");
  const kind = $derived(status.detectedKind || rule.kind);
  const forcedKind = $derived(Boolean(rule.kind && rule.kind !== AUTO_KIND));
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
  const sourceName = $derived(
    rule.source.include?.length
      ? t("rules.editor.selection_name", { count: rule.source.include.length })
      : baseName(rule.source.path) || rule.source.path
  );
  const sourceTitle = $derived(
    [
      t(`rules.origin.${rule.source.origin}`),
      sourcePath || rule.source.path,
      ...(rule.source.include ?? []),
    ].join("\n")
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

<article class="rule-card" class:rule-disabled={!rule.enabled} aria-label={rule.name}>
  <header class="card-head">
    <div class="title-col">
      <h3 class="rule-name" title={rule.name}>{rule.name}</h3>
      <div class="badges">
        <Badge tone={scope === "repo" ? "success" : "neutral"}>
          {t(`rules.scope.${scope}`)}
        </Badge>
        {#if kind}
          <Badge
            tone="info"
            title={forcedKind ? t("rules.card.kind_forced") : t("rules.card.kind_detected")}
          >
            {forcedKind
              ? t("rules.card.kind_forced_badge", { kind: t(`rules.kind.${kind}`) })
              : t(`rules.kind.${kind}`)}
          </Badge>
        {/if}
      </div>
    </div>

    <div class="head-actions">
      <div class="toggle">
        <Switch
          checked={rule.enabled}
          ariaLabel={t("rules.card.enabled")}
          onchange={(next) => ontoggle(next)}
        />
      </div>
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
  </header>

  <div class="source-row" title={sourceTitle}>
    <span class="origin-icon" aria-label={t(`rules.origin.${rule.source.origin}`)} role="img">
      <OriginIcon size={14} />
    </span>
    <span class="source-name">{sourceName}</span>
    {#if rule.source.origin === "subscription" && rule.source.subscription}
      <Badge mono>{rule.source.subscription}</Badge>
    {/if}
  </div>

  <div
    class="dest-section"
    aria-label={t("rules.card.destinations_label", { count: rule.destinations.length })}
  >
    {#each rule.destinations as destination (destination.id)}
      <DestinationStatusRow
        {rule}
        {destination}
        status={status.destinations.find((s) => s.destId === destination.id)}
        onreview={(file) => onreview(destination.id, file)}
        {onopen}
      />
    {/each}
  </div>
</article>

<style>
  .rule-card {
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

  .rule-card:hover {
    box-shadow: var(--elev-2);
  }

  .rule-disabled .rule-name,
  .rule-disabled .source-row,
  .rule-disabled .dest-section {
    opacity: 0.6;
  }

  .card-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }

  .title-col {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
    min-width: 0;
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

  .badges {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
    min-width: 0;
  }

  .head-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-shrink: 0;
  }

  .source-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    color: var(--text-muted);
  }

  .origin-icon {
    display: inline-flex;
    flex-shrink: 0;
  }

  .source-name {
    min-width: 0;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .dest-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
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
