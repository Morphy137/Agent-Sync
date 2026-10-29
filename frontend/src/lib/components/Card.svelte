<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    padding?: "sm" | "md" | "lg";
    interactive?: boolean;
    children?: Snippet;
  }

  let { padding = "md", interactive = false, children }: Props = $props();
</script>

<div class="card pad-{padding}" class:interactive>
  {@render children?.()}
</div>

<style>
  .card {
    background-color: var(--surface);
    border-radius: var(--radius-lg);
    box-shadow: var(--elev-1);
    transition: box-shadow var(--duration-fast) var(--easing-out);
  }

  .card.interactive:hover {
    box-shadow: var(--elev-2);
  }

  .pad-sm {
    padding: var(--space-3);
  }

  .pad-md {
    padding: var(--space-5);
  }

  .pad-lg {
    padding: var(--space-8);
  }
</style>
