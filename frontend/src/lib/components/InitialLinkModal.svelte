<script lang="ts">
  import type { main } from "../../../wailsjs/go/models";
  import { t } from "../i18n";
  import Modal from "./Modal.svelte";
  import Button from "./Button.svelte";

  interface Props {
    open: boolean;
    data: main.InitialLinkResult | null;
    onclose: () => void;
    onconfirm: (choice: string) => void;
  }

  let { open, data, onclose, onconfirm }: Props = $props();

  let choice = $state<string>("keep_repo");
</script>

<Modal {open} title={t("initial_link.title")} {onclose}>
  <div class="initial-link-container">
    <p class="desc">{t("initial_link.description")}</p>

    {#if data?.untrackedFiles && data.untrackedFiles.length > 0}
      <div class="diff-files">
        {#each data.untrackedFiles as file (file)}
          <div class="diff-file"><code>{file}</code></div>
        {/each}
      </div>
    {/if}

    <div class="options-list">
      <label class="option-card" class:selected={choice === "keep_repo"}>
        <input
          type="radio"
          name="initialChoice"
          value="keep_repo"
          checked={choice === "keep_repo"}
          onchange={() => (choice = "keep_repo")}
        />
        <div class="option-content">
          <div class="option-title">{t("initial_link.use_repo")}</div>
          <div class="option-desc">{t("initial_link.use_repo_desc")}</div>
        </div>
      </label>

      <label class="option-card" class:selected={choice === "keep_local"}>
        <input
          type="radio"
          name="initialChoice"
          value="keep_local"
          checked={choice === "keep_local"}
          onchange={() => (choice = "keep_local")}
        />
        <div class="option-content">
          <div class="option-title">{t("initial_link.use_local")}</div>
          <div class="option-desc">{t("initial_link.use_local_desc")}</div>
        </div>
      </label>

      <label class="option-card" class:selected={choice === "decide_later"}>
        <input
          type="radio"
          name="initialChoice"
          value="decide_later"
          checked={choice === "decide_later"}
          onchange={() => (choice = "decide_later")}
        />
        <div class="option-content">
          <div class="option-title">{t("initial_link.decide_later")}</div>
          <div class="option-desc">{t("initial_link.decide_later_desc")}</div>
        </div>
      </label>
    </div>

    <div class="footer-actions">
      <Button variant="secondary" onclick={onclose}>
        {t("common.cancel")}
      </Button>
      <Button variant="primary" onclick={() => onconfirm(choice)}>
        {t("initial_link.button_confirm")}
      </Button>
    </div>
  </div>
</Modal>

<style>
  .initial-link-container {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .desc {
    font-size: var(--text-sm);
    color: var(--text-muted);
    margin: 0;
    line-height: 1.5;
  }

  .diff-files {
    background-color: var(--surface-raised);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    border: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    max-height: 100px;
    overflow-y: auto;
    font-size: var(--text-xs);
  }

  .options-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .option-card {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    cursor: pointer;
    transition: all var(--duration-fast) var(--easing);
  }

  .option-card:hover {
    background-color: var(--surface-raised);
  }

  .option-card.selected {
    border-color: var(--accent);
    background-color: var(--surface-raised);
  }

  .option-content {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .option-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    color: var(--text);
  }

  .option-desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .footer-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-3);
    margin-top: var(--space-2);
  }
</style>
