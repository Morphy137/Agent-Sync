<script lang="ts">
  import { t } from "../i18n";

  interface Props {
    size?: "sm" | "md";
  }

  let { size = "md" }: Props = $props();
</script>

<div
  class="spinner spinner-{size}"
  role="status"
  aria-label={t("common.loading")}
></div>

<style>
  .spinner {
    border: 2px solid var(--hairline);
    border-top-color: currentColor;
    border-radius: var(--radius-full);
    animation: spin 0.8s linear infinite;
    display: inline-block;
    flex-shrink: 0;
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

  @media (prefers-reduced-motion: reduce) {
    .spinner {
      animation: none;
      opacity: 0.6;
    }
  }
</style>
