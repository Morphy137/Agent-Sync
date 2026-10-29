<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    padding?: "sm" | "md" | "lg";
    children?: Snippet;
  }

  let { padding = "md", children }: Props = $props();
</script>

<div class="card pad-{padding}">
  {@render children?.()}
</div>

<style>
  .card {
    background-color: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
    transition: border-color var(--duration-fast) var(--easing);
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
