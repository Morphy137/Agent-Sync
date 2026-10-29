<script lang="ts">
  import { ArrowDown, ArrowUp, AlertTriangle, ShieldCheck } from "@lucide/svelte";
  import type { orchestrator } from "../../../wailsjs/go/models";
  import { t } from "../i18n";
  import Modal from "./Modal.svelte";
  import Button from "./Button.svelte";

  interface Props {
    open: boolean;
    plan: orchestrator.SyncPlan | null;
    onclose: () => void;
    onconfirm: (resolutions: orchestrator.ConflictResolution[]) => void;
  }

  let { open, plan, onclose, onconfirm }: Props = $props();

  let resolutions = $state<Record<string, "keep_repo" | "keep_local">>({});

  $effect(() => {
    if (plan?.conflicts) {
      const initial: Record<string, "keep_repo" | "keep_local"> = {};
      for (const c of plan.conflicts) {
        const key = `${c.profile}:${c.relPath}`;
        initial[key] = resolutions[key] || "keep_repo";
      }
      resolutions = initial;
    }
  });

  function setAllResolutions(choice: "keep_repo" | "keep_local") {
    if (!plan?.conflicts) return;
    const updated: Record<string, "keep_repo" | "keep_local"> = {};
    for (const c of plan.conflicts) {
      updated[`${c.profile}:${c.relPath}`] = choice;
    }
    resolutions = updated;
  }

  function handleConfirm() {
    const list: orchestrator.ConflictResolution[] = [];
    if (plan?.conflicts) {
      for (const c of plan.conflicts) {
        const key = `${c.profile}:${c.relPath}`;
        list.push({
          profile: c.profile,
          relPath: c.relPath,
          resolution: resolutions[key] || "keep_repo",
        });
      }
    }
    onconfirm(list);
  }
</script>

<Modal {open} title={t("preview.title")} {onclose}>
  <div class="preview-container">
    {#if plan?.conflicts && plan.conflicts.length > 0}
      <section class="section conflicts-box">
        <div class="section-header danger">
          <div class="section-title">
            <AlertTriangle size={16} />
            <span>{t("preview.conflicts_section", { count: plan.conflicts.length })}</span>
          </div>
          <div class="quick-actions">
            <button
              type="button"
              class="link-btn"
              onclick={() => setAllResolutions("keep_repo")}
            >
              {t("conflicts.apply_all_repo")}
            </button>
            <span class="sep">|</span>
            <button
              type="button"
              class="link-btn"
              onclick={() => setAllResolutions("keep_local")}
            >
              {t("conflicts.apply_all_local")}
            </button>
          </div>
        </div>

        <div class="conflicts-list">
          {#each plan.conflicts as conflict (`${conflict.profile}:${conflict.relPath}`)}
            {@const key = `${conflict.profile}:${conflict.relPath}`}
            <div class="conflict-item">
              <span class="file-path">{conflict.profile}/{conflict.relPath}</span>
              <div class="radio-group">
                <label class="radio-label">
                  <input
                    type="radio"
                    name={key}
                    value="keep_repo"
                    checked={resolutions[key] === "keep_repo"}
                    onchange={() => (resolutions[key] = "keep_repo")}
                  />
                  <span>{t("conflicts.keep_repo")}</span>
                </label>
                <label class="radio-label">
                  <input
                    type="radio"
                    name={key}
                    value="keep_local"
                    checked={resolutions[key] === "keep_local"}
                    onchange={() => (resolutions[key] = "keep_local")}
                  />
                  <span>{t("conflicts.keep_local")}</span>
                </label>
              </div>
            </div>
          {/each}
        </div>
      </section>
    {/if}

    {#if plan?.downloads && plan.downloads.length > 0}
      <section class="section">
        <div class="section-title">
          <span class="icon-down"><ArrowDown size={16} /></span>
          <span>{t("preview.downloads_section", { count: plan.downloads.length })}</span>
        </div>
        <ul class="file-list">
          {#each plan.downloads as dl (dl.fullPath)}
            <li><code>{dl.fullPath}</code></li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if plan?.uploads && plan.uploads.length > 0}
      <section class="section">
        <div class="section-title">
          <span class="icon-up"><ArrowUp size={16} /></span>
          <span>{t("preview.uploads_section", { count: plan.uploads.length })}</span>
        </div>
        <ul class="file-list">
          {#each plan.uploads as up (up.fullPath)}
            <li><code>{up.fullPath}</code></li>
          {/each}
        </ul>
      </section>
    {/if}

    <div class="notice-bar">
      <span class="shield-icon"><ShieldCheck size={16} /></span>
      <span>{t("preview.backups_notice")}</span>
    </div>

    <div class="footer-actions">
      <Button variant="secondary" onclick={onclose}>
        {t("preview.button_cancel")}
      </Button>
      <Button variant="primary" onclick={handleConfirm}>
        {t("preview.button_sync")}
      </Button>
    </div>
  </div>
</Modal>

<style>
  .preview-container {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  .section {
    background-color: var(--surface-raised);
    border-radius: var(--radius-md);
    padding: var(--space-4);
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-3);
  }

  .section-title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--text);
  }

  .danger .section-title {
    color: var(--danger-fg);
  }

  .icon-down {
    color: var(--info-fg);
  }

  .icon-up {
    color: var(--accent);
  }

  .quick-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-xs);
  }

  .link-btn {
    background: none;
    border: none;
    color: var(--accent);
    cursor: pointer;
    padding: 0;
    text-decoration: underline;
  }

  .sep {
    color: var(--text-subtle);
  }

  .conflicts-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .conflict-item {
    background-color: var(--surface);
    padding: var(--space-3);
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
  }

  .file-path {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    display: block;
    margin-bottom: var(--space-2);
  }

  .radio-group {
    display: flex;
    gap: var(--space-4);
  }

  .radio-label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-xs);
    cursor: pointer;
  }

  .file-list {
    margin: var(--space-2) 0 0 0;
    padding: 0 0 0 var(--space-5);
    font-size: var(--text-xs);
    color: var(--text);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    max-height: 120px;
    overflow-y: auto;
  }

  .notice-bar {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-muted);
    padding: var(--space-2) 0;
  }

  .shield-icon {
    color: var(--success-fg);
  }

  .footer-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-3);
    margin-top: var(--space-2);
  }
</style>
