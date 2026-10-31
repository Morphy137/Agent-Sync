<script lang="ts">
  import { ArrowRight } from "@lucide/svelte";
  import type { rules } from "../../../../wailsjs/go/models";
  import { t } from "../../i18n";
  import { appState } from "../../stores/appState.svelte";
  import Badge from "./Badge.svelte";
  import { destinationLabel } from "./ruleUi";

  interface Props {
    collisions: rules.Collision[];
    /** Rules used to turn references into readable names. */
    knownRules: rules.Rule[];
    /** When set, each rule reference gets an "Edit rule" action. */
    onedit?: (ruleId: string) => void;
  }

  let { collisions, knownRules, onedit }: Props = $props();

  interface RefView {
    key: string;
    ruleId: string;
    ruleName: string;
    target: string;
  }

  function describeRef(ref: rules.DestRef, index: number): RefView {
    if (ref.connectionId) {
      const conn = appState.config?.connections?.find((c) => c.id === ref.connectionId);
      return {
        key: `c-${ref.connectionId}-${index}`,
        ruleId: "",
        ruleName: t("rules.collisions.connection_ref", {
          profile: conn?.profile ?? ref.connectionId,
        }),
        target: conn?.targetPath ?? "",
      };
    }
    const rule = knownRules.find((r) => r.id === ref.ruleId);
    const dest = rule?.destinations.find((d) => d.id === ref.destId);
    return {
      key: `r-${ref.ruleId}-${ref.destId}-${index}`,
      ruleId: ref.ruleId ?? "",
      ruleName: rule?.name ?? ref.ruleId ?? "",
      target: dest ? destinationLabel(dest) : (ref.destId ?? ""),
    };
  }
</script>

<ul class="collision-list">
  {#each collisions as collision, index (`${collision.type}-${collision.path}-${index}`)}
    <li class="collision-item">
      <div class="collision-head">
        <Badge tone="danger">{t(`rules.collisions.type.${collision.type}`)}</Badge>
        <span class="collision-path" title={collision.path}>{collision.path}</span>
      </div>
      <p class="collision-hint">{t(`rules.collisions.hint.${collision.type}`)}</p>
      <ul class="ref-list" aria-label={t("rules.collisions.involved")}>
        {#each (collision.refs ?? []).map(describeRef) as ref (ref.key)}
          <li class="ref-row">
            <span class="ref-rule">{ref.ruleName}</span>
            {#if ref.target}
              <ArrowRight size={12} aria-hidden="true" />
              <span class="ref-target">{ref.target}</span>
            {/if}
            {#if onedit && ref.ruleId}
              <button type="button" class="ref-edit" onclick={() => onedit?.(ref.ruleId)}>
                {t("rules.collisions.edit_rule", { name: ref.ruleName })}
              </button>
            {/if}
          </li>
        {/each}
      </ul>
    </li>
  {/each}
</ul>

<style>
  .collision-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .collision-item {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .collision-head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }

  .collision-path {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .collision-hint {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .ref-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .ref-row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-1) var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .ref-rule {
    font-weight: var(--weight-medium);
    color: var(--ink);
  }

  .ref-target {
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }

  .ref-edit {
    margin-left: auto;
    background: none;
    border: none;
    padding: 0 var(--space-1);
    border-radius: var(--radius-xs);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--ink);
    text-decoration: underline;
    text-underline-offset: 2px;
    cursor: pointer;
  }

  .ref-edit:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }
</style>
