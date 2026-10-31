<script lang="ts">
  import { FolderMinus, FileMinus, ShieldCheck } from "@lucide/svelte";
  import type { service } from "../../../wailsjs/go/models";
  import { api } from "../api";
  import { t } from "../i18n";
  import { appState } from "../stores/appState.svelte";
  import Modal from "./Modal.svelte";
  import Button from "./Button.svelte";
  import Spinner from "./Spinner.svelte";

  interface Props {
    open: boolean;
    onclose: () => void;
  }

  let { open, onclose }: Props = $props();

  const editorId = "gitignore-editor";

  let payload = $state<service.GitignorePayload | null>(null);
  let draft = $state("");
  let loading = $state(false);
  let busy = $state(false);

  const dirty = $derived(payload !== null && draft !== payload.content);
  const missingCount = $derived(payload?.missingRecommended?.length ?? 0);

  $effect(() => {
    if (open) void load();
  });

  function apply(next: service.GitignorePayload) {
    payload = next;
    draft = next.content;
  }

  async function load() {
    loading = true;
    try {
      apply(await api.getGitignore());
    } catch (err) {
      appState.showError(err);
    } finally {
      loading = false;
    }
  }

  async function run(action: () => Promise<service.GitignorePayload | null>, toast?: string) {
    busy = true;
    try {
      const next = await action();
      if (!next) return;
      apply(next);
      if (toast) appState.addToast("success", toast);
    } catch (err) {
      appState.showError(err);
    } finally {
      busy = false;
    }
  }

  async function exclude(kind: "folder" | "file") {
    const root = appState.config?.rootPath ?? "";
    const picked =
      kind === "folder"
        ? await api.selectFolder(t("exclusions.pick_folder_title"), root)
        : await api.selectFile(t("exclusions.pick_file_title"), root);
    if (!picked) return null;
    const next = await api.excludeRepoPath(picked);
    const added = next.content.trim().split("\n").at(-1) ?? picked;
    appState.addToast("success", t("exclusions.toast_added", { pattern: added }));
    return next;
  }

  function handleSave() {
    void run(() => api.saveGitignore(draft), t("exclusions.toast_saved"));
  }
</script>

<Modal {open} title={t("exclusions.title")} size="lg" {onclose}>
  <div class="exclusions">
    <p class="description">{t("exclusions.description")}</p>

    <div class="toolbar">
      <div class="recommended" class:ok={missingCount === 0}>
        <ShieldCheck size={16} />
        <span>
          {missingCount > 0
            ? t("exclusions.recommended_missing", { count: missingCount })
            : t("exclusions.recommended_ok")}
        </span>
      </div>
      <div class="toolbar-actions">
        {#if missingCount > 0}
          <Button
            variant="secondary"
            size="sm"
            disabled={busy || dirty}
            onclick={() =>
              run(() => api.addRecommendedGitignore(), t("exclusions.toast_recommended"))}
          >
            {t("exclusions.add_recommended")}
          </Button>
        {/if}
        <Button
          variant="secondary"
          size="sm"
          disabled={busy || dirty}
          onclick={() => run(() => exclude("folder"))}
        >
          <FolderMinus size={14} />
          <span>{t("exclusions.exclude_folder")}</span>
        </Button>
        <Button
          variant="secondary"
          size="sm"
          disabled={busy || dirty}
          onclick={() => run(() => exclude("file"))}
        >
          <FileMinus size={14} />
          <span>{t("exclusions.exclude_file")}</span>
        </Button>
      </div>
    </div>

    <label class="editor-label" for={editorId}>{t("exclusions.editor_label")}</label>
    <div class="editor-frame">
      {#if loading}
        <div class="editor-loading"><Spinner /></div>
      {:else}
        <textarea id={editorId} class="editor" spellcheck="false" bind:value={draft}></textarea>
      {/if}
    </div>

    <p class="note">{dirty ? t("exclusions.unsaved_note") : t("exclusions.tracked_note")}</p>

    <div class="actions">
      <Button variant="ghost" onclick={onclose}>{t("common.close")}</Button>
      <Button variant="primary" disabled={!dirty || busy} loading={busy} onclick={handleSave}>
        {t("common.save")}
      </Button>
    </div>
  </div>
</Modal>

<style>
  .exclusions {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .description,
  .note {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
    line-height: 1.5;
  }

  .note {
    font-size: var(--text-xs);
    min-height: 1.5em;
  }

  .toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .toolbar-actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .recommended {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-xs);
    color: var(--dot-up);
  }

  .recommended.ok {
    color: var(--dot-ok);
  }

  .editor-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .editor-frame {
    height: 320px;
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    overflow: hidden;
  }

  .editor {
    width: 100%;
    height: 100%;
    padding: var(--space-3);
    border: none;
    resize: none;
    background: transparent;
    color: var(--ink);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    line-height: 1.6;
    outline: none;
  }

  .editor:focus-visible {
    box-shadow: inset var(--focus-ring);
  }

  .editor-loading {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }
</style>
