<script lang="ts">
  import { untrack } from "svelte";
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

  /** Keeps a decision per conflicting file, defaulting new ones to the repository version. */
  $effect(() => {
    const conflicts = plan?.conflicts;
    if (!conflicts) return;
    untrack(() => {
      const initial: Record<string, "keep_repo" | "keep_local"> = {};
      for (const c of conflicts) {
        initial[c.path] = resolutions[c.path] || "keep_repo";
      }
      resolutions = initial;
    });
  });

  function setAllResolutions(choice: "keep_repo" | "keep_local") {
    if (!plan?.conflicts) return;
    const updated: Record<string, "keep_repo" | "keep_local"> = {};
    for (const c of plan.conflicts) {
      updated[c.path] = choice;
    }
    resolutions = updated;
  }

  function handleConfirm() {
    if (syncing) return;
    const list: orchestrator.ConflictResolution[] = [];
    if (plan?.conflicts) {
      for (const c of plan.conflicts) {
        list.push({ path: c.path, resolution: resolutions[c.path] || "keep_repo" });
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
          {#each plan.conflicts as conflict (conflict.path)}
            <div class="conflict-row">
              <span class="file-path">{conflict.path}</span>
              <div class="conflict-control">
                <SegmentedControl
                  size="sm"
                  ariaLabel={conflict.path}
                  value={resolutions[conflict.path] || "keep_repo"}
                  options={[
                    { value: "keep_repo", label: t("conflicts.keep_repo") },
                    { value: "keep_local", label: t("conflicts.keep_local") },
                  ]}
                  onchange={(val) => (resolutions[conflict.path] = val as "keep_repo" | "keep_local")}
                />
              </div>
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
