<script lang="ts">
  import type { rules } from "../../../../wailsjs/go/models";
  import { t } from "../../i18n";
  import CollisionList from "./CollisionList.svelte";
  import StatusDot from "./StatusDot.svelte";

  interface Props {
    collisions: rules.Collision[];
    knownRules: rules.Rule[];
    onedit?: (ruleId: string) => void;
  }

  let { collisions, knownRules, onedit }: Props = $props();
</script>

<section class="collision-banner" role="alert" aria-labelledby="collision-banner-title">
  <header class="banner-head">
    <StatusDot tone="conflict" />
    <h3 id="collision-banner-title" class="banner-title">
      {t("rules.collisions.banner_title", { count: collisions.length })}
    </h3>
  </header>
  <p class="banner-message">{t("rules.collisions.banner_message")}</p>
  <CollisionList {collisions} {knownRules} {onedit} />
</section>

<style>
  .collision-banner {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-lg);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
  }

  .banner-head {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .banner-title {
    margin: 0;
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .banner-message {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
</style>
