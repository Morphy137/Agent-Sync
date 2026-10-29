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
    <div class="icon-wrap">
      <FolderGit2 size={48} />
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
    min-height: calc(100vh - 120px);
    padding: var(--space-6);
  }

  .welcome-card {
    background-color: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-md);
    padding: var(--space-10);
    max-width: 520px;
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .icon-wrap {
    color: var(--accent);
    margin-bottom: var(--space-5);
  }

  .welcome-title {
    font-size: var(--text-2xl);
    font-weight: var(--weight-bold);
    color: var(--text);
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
