<script lang="ts">
  import { onMount } from "svelte";
  import { Bold, Italic, Heading, FolderOpen } from "@lucide/svelte";
  import type { rulesync } from "../../../wailsjs/go/models";
  import { api, parseError } from "../api";
  import { appState } from "../stores/appState.svelte";
  import { t } from "../i18n";
  import Button from "../components/Button.svelte";
  import IconButton from "../components/IconButton.svelte";
  import Spinner from "../components/Spinner.svelte";

  type SourceFile = { path: string; name: string; absolutePath: string };
  let rules = $state<rulesync.RuleStatus[]>([]);
  let selectedRule = $state("");
  let files = $state<SourceFile[]>([]);
  let selected = $state<SourceFile | null>(null);
  let content = $state("");
  let original = $state("");
  let hash = $state("");
  let usesCrLf = false;
  let readOnly = $state(false);
  let loading = $state(true);
  let busy = $state(false);
  let editor = $state<HTMLTextAreaElement>();
  let request = 0;

  $effect(() => {
    appState.fileDirty = content !== original;
  });

  function allowLeaving() {
    return !appState.fileDirty || window.confirm(t("files.unsaved_confirm"));
  }

  onMount(() => {
    void loadRules();
    const warn = (event: BeforeUnloadEvent) => {
      if (appState.fileDirty) {
        event.preventDefault();
        event.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  });

  async function loadRules() {
    try {
      const report = await api.getRulesReport();
      rules = report.rules ?? [];
      if (appState.fileRuleId && rules.some((item) => item.rule.id === appState.fileRuleId)) {
        await selectRule(appState.fileRuleId, true);
      }
    } catch (err) {
      appState.showError(err);
    } finally {
      loading = false;
    }
  }

  async function selectRule(id: string, initial = false) {
    if (!initial && !allowLeaving()) return;
    const current = ++request;
    selectedRule = id;
    appState.fileRuleId = id;
    selected = null;
    content = original = "";
    files = [];
    busy = true;
    try {
      const result = await api.listRuleSourceFiles(id);
      if (current !== request) return;
      files = result ?? [];
      if (files.length === 1 && files[0].path === ".") await selectFile(files[0], true);
    } catch (err) {
      if (current === request) appState.showError(err);
    } finally {
      if (current === request) busy = false;
    }
  }

  async function selectFile(file: SourceFile, initial = false) {
    if (!initial && !allowLeaving()) return;
    const current = ++request;
    busy = true;
    try {
      const result = await api.readRuleSourceFile(selectedRule, file.path);
      if (current !== request) return;
      selected = file;
      usesCrLf =
        result.content.includes("\r\n") && !result.content.replaceAll("\r\n", "").includes("\n");
      content = original = result.content.replaceAll("\r\n", "\n");
      hash = result.hash;
      readOnly = result.readOnly;
    } catch (err) {
      if (current === request) appState.showError(err);
    } finally {
      if (current === request) busy = false;
    }
  }

  async function save() {
    if (!selected || busy || readOnly) return;
    busy = true;
    try {
      const saved = content;
      hash = await api.saveRuleSourceFile(
        selectedRule,
        selected.path,
        usesCrLf ? saved.replaceAll("\n", "\r\n") : saved,
        hash
      );
      original = saved;
      appState.addToast("success", t("files.saved"));
      appState.bumpRules();
    } catch (err) {
      if (parseError(err).code === "plan_changed")
        appState.addToast("error", t("files.changed_externally"));
      else appState.showError(err);
    } finally {
      busy = false;
    }
  }

  function format(prefix: string, suffix = prefix) {
    if (!editor) return;
    const start = editor.selectionStart;
    const end = editor.selectionEnd;
    content =
      content.slice(0, start) + prefix + content.slice(start, end) + suffix + content.slice(end);
    editor.focus();
    requestAnimationFrame(() =>
      editor?.setSelectionRange(start + prefix.length, end + prefix.length)
    );
  }

  async function openExplorer() {
    if (!selected) return;
    try {
      await api.openInExplorer(selected.absolutePath.replace(/[\\/][^\\/]+$/, ""));
    } catch (err) {
      appState.showError(err);
    }
  }
</script>

<section class="view">
  <header>
    <h2>{t("files.title")}</h2>
    <p>{t("files.intro")}</p>
  </header>
  {#if loading}<Spinner size="sm" />{:else}
    <div class="workspace">
      <nav aria-label={t("files.sources")} class="sources">
        <h3>{t("files.sources")}</h3>
        {#each rules as item (item.rule.id)}
          <button
            type="button"
            class:active={selectedRule === item.rule.id}
            onclick={() => selectRule(item.rule.id)}
            title={item.rule.source.path}
          >
            <span class="source-label">{item.rule.name}</span>
            <span class="source-path">{item.rule.source.path}</span>
          </button>
        {/each}
        {#if rules.length === 0}<p>{t("files.no_rules")}</p>{/if}
      </nav>
      <nav aria-label={t("files.list")} class="file-list">
        <h3>{t("files.list")}</h3>
        {#each files as file (file.path)}
          <button
            type="button"
            class:active={selected?.path === file.path}
            onclick={() => selectFile(file)}
            title={file.path}>{file.path === "." ? file.name : file.path}</button
          >
        {/each}
        {#if selectedRule && files.length === 0 && !busy}<p>{t("files.no_files")}</p>{/if}
      </nav>
      <div class="editor-panel">
        {#if selected}
          <div class="toolbar">
            <strong title={selected.absolutePath}>{selected.name}</strong>
            {#if readOnly}<span>{t("files.read_only")}</span>{/if}
            <div class="actions">
              {#if !readOnly && selected.name.toLowerCase().endsWith(".md")}
                <IconButton label={t("files.bold")} disabled={busy} onclick={() => format("**")}
                  ><Bold size={16} /></IconButton
                >
                <IconButton label={t("files.italic")} disabled={busy} onclick={() => format("*")}
                  ><Italic size={16} /></IconButton
                >
                <IconButton
                  label={t("files.heading")}
                  disabled={busy}
                  onclick={() => format("## ", "")}><Heading size={16} /></IconButton
                >
              {/if}
              <IconButton label={t("files.open_explorer")} onclick={openExplorer}
                ><FolderOpen size={16} /></IconButton
              >
              {#if !readOnly}<Button
                  size="sm"
                  variant="primary"
                  disabled={busy || !appState.fileDirty}
                  onclick={save}>{t("files.save")}</Button
                >{/if}
            </div>
          </div>
          <textarea
            bind:this={editor}
            bind:value={content}
            readonly={readOnly || busy}
            aria-label={t("files.editor_label", { name: selected.name })}
            spellcheck="true"></textarea>
        {:else}<p class="hint">
            {t(selectedRule ? "files.select_file" : "files.select_source")}
          </p>{/if}
      </div>
    </div>
  {/if}
</section>

<style>
  .view {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    flex: 1;
    min-width: 0;
  }
  h2 {
    margin: 0;
    color: var(--ink);
    font-size: var(--text-lg);
  }
  header p,
  p {
    color: var(--text-muted);
    font-size: var(--text-sm);
  }
  .workspace {
    display: grid;
    grid-template-columns: minmax(140px, 1fr) minmax(140px, 1fr) minmax(300px, 3fr);
    min-height: 480px;
    gap: var(--space-3);
  }
  nav,
  .editor-panel {
    background: var(--surface);
    border-radius: var(--radius-lg);
    box-shadow: var(--elev-1);
    padding: var(--space-3);
    min-width: 0;
  }
  nav {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    overflow: auto;
  }
  h3 {
    color: var(--ink);
    font-size: var(--text-sm);
    margin: 0 0 var(--space-2);
  }
  nav button {
    text-align: left;
    border: 0;
    border-radius: var(--radius-sm);
    padding: var(--space-2);
    color: var(--ink);
    background: transparent;
    cursor: pointer;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  nav button:hover,
  nav button.active {
    background: var(--surface-sunken);
  }
  nav button:focus-visible {
    outline: 2px solid var(--ink);
  }
  .source-label,
  .source-path {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .source-path {
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-family: var(--font-mono);
  }
  .editor-panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .toolbar,
  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .toolbar {
    flex-wrap: wrap;
    min-height: var(--space-8);
  }
  .toolbar strong {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .toolbar span {
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
  .actions {
    margin-left: auto;
  }
  textarea {
    flex: 1;
    min-height: 360px;
    width: 100%;
    box-sizing: border-box;
    resize: vertical;
    padding: var(--space-3);
    background: var(--surface-sunken);
    color: var(--ink);
    border: 0;
    border-radius: var(--radius-md);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    line-height: 1.6;
  }
  .hint {
    margin: auto;
    text-align: center;
  }
  @media (max-width: 850px) {
    .workspace {
      grid-template-columns: 1fr 1fr;
    }
    .editor-panel {
      grid-column: 1 / -1;
    }
  }
</style>
