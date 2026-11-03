<script lang="ts">
  import { Eye } from "@lucide/svelte";
  import { t } from "../../i18n";
  import Button from "../Button.svelte";

  /** The incoming files of a subscription; the card's expand control shows and hides it. */
  interface Props {
    id: string;
    files: string[];
    onviewdiff: (path: string) => void;
  }

  let { id, files, onviewdiff }: Props = $props();
</script>

<ul class="incoming-list" {id}>
  {#each files as path (path)}
    <li class="incoming-row">
      <span class="incoming-path" title={path}>{path}</span>
      <Button
        size="sm"
        variant="ghost"
        ariaLabel={t("subscriptions.incoming.view_diff_for", { path })}
        onclick={() => onviewdiff(path)}
      >
        <Eye size={14} />
        <span>{t("subscriptions.incoming.view_diff")}</span>
      </Button>
    </li>
  {/each}
</ul>

<style>
  .incoming-list {
    list-style: none;
    margin: 0;
    padding: var(--space-1) var(--space-3);
    max-height: 180px;
    overflow-y: auto;
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-md);
  }

  .incoming-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    min-height: 34px;
  }

  .incoming-path {
    min-width: 0;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
