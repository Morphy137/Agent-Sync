<script lang="ts">
  import { FolderGit2 } from "@lucide/svelte";
  import { t } from "../i18n";
  import Button from "./Button.svelte";

  interface Props {
    onselectFolder: () => void;
  }

  let { onselectFolder }: Props = $props();
</script>

<div class="welcome-container">
  <div class="welcome-card">
    <div class="icon-circle">
      <FolderGit2 size={28} />
    </div>
    <h1 class="welcome-title">{t("welcome.title")}</h1>
    <p class="welcome-desc">{t("welcome.description")}</p>
    <div class="action-wrap">
      <Button variant="primary" size="md" onclick={onselectFolder}>
        {t("welcome.button_select_folder")}
      </Button>
    </div>
  </div>
</div>

<style>
  .welcome-container {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100%;
    padding: var(--space-6);
  }

  .welcome-card {
    background-color: var(--surface);
    border-radius: var(--radius-xl);
    box-shadow: var(--elev-3);
    padding: var(--space-10);
    max-width: 520px;
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .icon-circle {
    width: 64px;
    height: 64px;
    border-radius: var(--radius-full);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--ink);
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: var(--space-6);
  }

  .welcome-title {
    font-size: var(--text-2xl);
    font-weight: var(--weight-bold);
    color: var(--ink);
    margin: 0 0 var(--space-3) 0;
  }

  .welcome-desc {
    font-size: var(--text-sm);
    color: var(--text-muted);
    line-height: 1.6;
    margin: 0 0 var(--space-8) 0;
  }

  .action-wrap {
    display: flex;
  }
</style>
