<script lang="ts">
  interface Props {
    size?: "sm" | "md";
  }

  let { size = "md" }: Props = $props();
</script>

<div class="spinner spinner-{size}" role="status" aria-label="Loading"></div>

<style>
  .spinner {
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: var(--radius-full);
    animation: spin 0.8s linear infinite;
    display: inline-block;
  }

  .spinner-sm {
    width: 14px;
    height: 14px;
    border-width: 1.5px;
  }

  .spinner-md {
    width: 20px;
    height: 20px;
    border-width: 2px;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
