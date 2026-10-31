<script lang="ts">
  import { api } from "../api";
  import { parseChangelog, toSegments, type ChangelogRelease } from "../changelog/parse";
  import { t } from "../i18n";
  import Modal from "./Modal.svelte";
  import Spinner from "./Spinner.svelte";

  interface Props {
    open: boolean;
    onclose: () => void;
  }

  let { open, onclose }: Props = $props();

  let releases = $state<ChangelogRelease[]>([]);
  let loaded = $state(false);

  /** Known group names get a translated title; anything else is shown as written. */
  const KNOWN_GROUPS = ["added", "changed", "fixed", "removed", "security", "deprecated"];

  function groupTitle(title: string): string {
    const key = title.toLowerCase();
    return KNOWN_GROUPS.includes(key) ? t(`changelog.groups.${key}`) : title;
  }

  $effect(() => {
    if (!open || loaded) return;
    void api
      .getChangelog()
      .then((markdown) => (releases = parseChangelog(markdown)))
      .catch(() => (releases = []))
      .finally(() => (loaded = true));
  });
</script>

<Modal {open} title={t("changelog.title")} size="lg" {onclose}>
  {#if !loaded}
    <div class="state"><Spinner /></div>
  {:else if releases.length === 0}
    <p class="state">{t("changelog.empty")}</p>
  {:else}
    <div class="releases">
      {#each releases as release (release.version)}
        <section class="release">
          <header class="release-head">
            <h3 class="release-version">
              {release.unreleased
                ? t("changelog.unreleased")
                : t("changelog.version", { version: release.version })}
            </h3>
            {#if release.date}<span class="release-date">{release.date}</span>{/if}
          </header>
          {#each release.sections as section (section.title)}
            {#if section.items.length > 0}
              <div class="group">
                <h4 class="group-title group-{section.title.toLowerCase()}">
                  {groupTitle(section.title)}
                </h4>
                <ul class="items">
                  {#each section.items as item, i (i)}
                    <li>
                      {#each toSegments(item) as segment, j (j)}
                        {#if segment.code}<code>{segment.text}</code>{:else}{segment.text}{/if}
                      {/each}
                    </li>
                  {/each}
                </ul>
              </div>
            {/if}
          {/each}
        </section>
      {/each}
    </div>
  {/if}
</Modal>

<style>
  .state {
    display: flex;
    justify-content: center;
    padding: var(--space-8) 0;
    margin: 0;
    color: var(--text-muted);
    font-size: var(--text-sm);
  }

  .releases {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .release {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .release-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-3);
  }

  .release-version {
    margin: 0;
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .release-date {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .group-title {
    margin: 0;
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-muted);
  }

  .group-added {
    color: var(--dot-ok);
  }

  .group-fixed {
    color: var(--dot-down);
  }

  .group-removed {
    color: var(--dot-conflict);
  }

  .items {
    margin: 0;
    padding-left: var(--space-5);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    font-size: var(--text-sm);
    line-height: 1.5;
    color: var(--ink);
  }

  code {
    font-family: var(--font-mono);
    font-size: 0.92em;
    padding: 0 4px;
    border-radius: var(--radius-xs);
    background-color: var(--surface);
  }
</style>
