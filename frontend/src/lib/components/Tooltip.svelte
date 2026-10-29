<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    text: string;
    children?: Snippet;
  }

  let { text, children }: Props = $props();
</script>

<div class="tooltip-wrapper" title={text}>
  {@render children?.()}
</div>

<style>
  .tooltip-wrapper {
    display: inline-flex;
    max-width: 100%;
    vertical-align: middle;
  }
</style>
