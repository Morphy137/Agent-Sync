<script lang="ts">
  import { ShieldCheck } from "@lucide/svelte";
  import type { orchestrator } from "../../../wailsjs/go/models";
  import { t } from "../i18n";
  import Modal from "./Modal.svelte";
  import Button from "./Button.svelte";
  import SegmentedControl from "./SegmentedControl.svelte";

  interface Props {
    open: boolean;
    plan: orchestrator.SyncPlan | null;
    syncing?: boolean;
    onclose: () => void;
    onconfirm: (resolutions: orchestrator.ConflictResolution[]) => void;
  }

  let { open, plan, syncing = false, onclose, onconfirm }: Props = $props();

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
    if (syncing) return;
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
      <section class="group-section">
        <div class="section-header">
          <div class="section-title">
            <span class="status-dot dot-conflict"></span>
            <span>{t("preview.conflicts_section", { count: plan.conflicts.length })}</span>
          </div>
          <div class="quick-actions">
            <Button size="sm" variant="ghost" onclick={() => setAllResolutions("keep_repo")}>
              {t("conflicts.apply_all_repo")}
            </Button>
            <Button size="sm" variant="ghost" onclick={() => setAllResolutions("keep_local")}>
              {t("conflicts.apply_all_local")}
            </Button>
          </div>
        </div>

        <div class="sunken-panel conflicts-list">
          {#each plan.conflicts as conflict (`${conflict.profile}:${conflict.relPath}`)}
            {@const key = `${conflict.profile}:${conflict.relPath}`}
            <div class="conflict-row">
              <span class="file-path">{conflict.profile}/{conflict.relPath}</span>
              <div class="conflict-control">
                <SegmentedControl
                  size="sm"
                  ariaLabel={`${conflict.profile}/${conflict.relPath}`}
                  value={resolutions[key] || "keep_repo"}
                  options={[
                    { value: "keep_repo", label: t("conflicts.keep_repo") },
                    { value: "keep_local", label: t("conflicts.keep_local") },
                  ]}
                  onchange={(val) => (resolutions[key] = val as "keep_repo" | "keep_local")}
                />
              </div>
            </div>
          {/each}
        </div>
      </section>
    {/if}

    {#if plan?.downloads && plan.downloads.length > 0}
      <section class="group-section">
        <div class="section-header">
          <div class="section-title">
            <span class="status-dot dot-down"></span>
            <span>{t("preview.downloads_section", { count: plan.downloads.length })}</span>
          </div>
        </div>
        <div class="sunken-panel file-list">
          {#each plan.downloads as dl (dl.fullPath)}
            <div class="file-item">
              <span class="file-path">{dl.fullPath}</span>
            </div>
          {/each}
        </div>
      </section>
    {/if}

    {#if plan?.uploads && plan.uploads.length > 0}
      <section class="group-section">
        <div class="section-header">
          <div class="section-title">
            <span class="status-dot dot-up"></span>
            <span>{t("preview.uploads_section", { count: plan.uploads.length })}</span>
          </div>
        </div>
        <div class="sunken-panel file-list">
          {#each plan.uploads as up (up.fullPath)}
            <div class="file-item">
              <span class="file-path">{up.fullPath}</span>
            </div>
          {/each}
        </div>
      </section>
    {/if}

    <div class="notice-bar">
      <ShieldCheck size={15} />
      <span>{t("preview.backups_notice")}</span>
    </div>

    <div class="footer-actions">
      <Button variant="secondary" onclick={onclose}>
        {t("preview.button_cancel")}
      </Button>
      <Button variant="primary" loading={syncing} onclick={handleConfirm}>
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

  .group-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .section-title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: var(--radius-full);
    flex-shrink: 0;
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

  .quick-actions {
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }

  .sunken-panel {
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-md);
    padding: var(--space-2) var(--space-3);
  }

  .conflicts-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .conflict-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-1) 0;
  }

  .conflict-control {
    width: 210px;
    flex-shrink: 0;
  }

  .file-list {
    display: flex;
    flex-direction: column;
    max-height: 140px;
    overflow-y: auto;
  }

  .file-item {
    height: 28px;
    display: flex;
    align-items: center;
  }

  .file-path {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .notice-bar {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .footer-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-3);
  }
</style>
