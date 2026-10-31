<script lang="ts">
  import type { DotTone } from "./ruleUi";

  interface Props {
    tone: DotTone;
  }

  let { tone }: Props = $props();
</script>

<span class="dot dot-{tone}" aria-hidden="true"></span>

<style>
  .dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: var(--radius-full);
    flex-shrink: 0;
  }

  .dot-ok {
    background-color: var(--dot-ok);
  }

  .dot-down {
    background-color: var(--dot-down);
  }

  .dot-up {
    background-color: var(--dot-up);
  }

  .dot-conflict {
    background-color: var(--dot-conflict);
  }

  .dot-idle {
    background-color: var(--dot-idle);
  }
</style>
