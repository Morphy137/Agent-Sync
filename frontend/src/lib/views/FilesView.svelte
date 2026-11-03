<script lang="ts">
  import { onMount } from "svelte";
  import {
    Bold,
    Italic,
    Heading,
    FolderOpen,
    Folder,
    FileText,
    Eye,
    ArrowLeft,
    Home,
    RefreshCw,
    Braces,
  } from "@lucide/svelte";
  import { api, parseError } from "../api";
  import { appState } from "../stores/appState.svelte";
  import { t } from "../i18n";
  import { supportsPreview } from "../fileKinds";
  import Button from "../components/Button.svelte";
  import IconButton from "../components/IconButton.svelte";
  import MarkdownPreview from "../components/MarkdownPreview.svelte";
  import Spinner from "../components/Spinner.svelte";
  import ViewHeader from "../components/ViewHeader.svelte";

  type Root = { id: string; name: string };
  type Entry = { path: string; name: string; isDir: boolean };
  type SelectedFile = { rootId: string; path: string; name: string; absolutePath: string };
  let roots = $state<Root[]>([]);
  let rootId = $state("");
  let directory = $state(".");
  let entries = $state<Entry[]>([]);
  let selected = $state<SelectedFile | null>(null);
  let content = $state("");
  let original = $state("");
  let hash = $state("");
  let usesCrLf = false;
  let readOnly = $state(false);
  /** Markdown opens as a rendered preview; the eye toggle switches this file to the editor. */
  let editing = $state(false);
  let loading = $state(true);
  let busy = $state(false);
  let editor = $state<HTMLTextAreaElement>();
  let request = 0;
  const isMarkdown = $derived(supportsPreview(selected?.name ?? ""));
  const isJson = $derived(/\.json$/i.test(selected?.name ?? ""));
  const rootName = $derived(roots.find((root) => root.id === rootId)?.name ?? rootId);
  const preview = $derived(isMarkdown && !editing);

  $effect(() => {
    appState.fileDirty = content !== original;
  });

  function allowLeaving() {
    return !appState.fileDirty || window.confirm(t("files.unsaved_confirm"));
  }

  function showFileError(err: unknown) {
    const reason = parseError(err).params?.reason;
    if (reason === "file_too_large" || reason === "file_not_text") {
      appState.addToast("error", t(`files.${reason}`));
    } else appState.showError(err);
  }

  onMount(() => {
    void initialise();
    const warn = (event: BeforeUnloadEvent) => {
      if (appState.fileDirty) {
        event.preventDefault();
        event.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  });

  async function initialise() {
    try {
      roots = await api.listFileRoots();
      const id = appState.fileRuleId;
      appState.fileRuleId = "";
      if (id) {
        const report = await api.getRulesReport();
        const rule = report.rules.find((item) => item.rule.id === id)?.rule;
        if (!rule) return;
        const root =
          rule.source.origin === "repo"
            ? "repo"
            : rule.source.origin === "subscription"
              ? rule.source.subscription
              : `rule:${id}`;
        if (!root) return;
        if (rule.source.origin === "local") roots = [...roots, { id: root, name: rule.name }];
        const sourceFiles = await api.listRuleSourceFiles(id);
        const isFile = sourceFiles.length === 1 && sourceFiles[0].path === ".";
        const path =
          rule.source.origin === "local" ? "." : (rule.source.path || ".").replaceAll("\\", "/");
        const parent =
          isFile && path !== "."
            ? path.includes("/")
              ? path.slice(0, path.lastIndexOf("/"))
              : "."
            : path;
        await openDirectory(root, parent);
        if (isFile) await openFile(root, path, sourceFiles[0].name, true);
      }
    } catch (err) {
      appState.showError(err);
    } finally {
      loading = false;
    }
  }

  async function openDirectory(root: string, path = ".") {
    const current = ++request;
    busy = true;
    try {
      const result = await api.listFileDirectory(root, path);
      if (current !== request) return;
      rootId = root;
      directory = path;
      entries = result ?? [];
    } catch (err) {
      if (current === request) showFileError(err);
    } finally {
      if (current === request) busy = false;
    }
  }

  async function openFile(root: string, path: string, name: string, initial = false) {
    if (!initial && !allowLeaving()) return;
    const current = ++request;
    busy = true;
    try {
      const result = await api.readBrowserFile(root, path);
      if (current !== request) return;
      selected = { rootId: root, path, name, absolutePath: result.absolutePath };
      editing = false;
      usesCrLf =
        result.content.includes("\r\n") && !result.content.replaceAll("\r\n", "").includes("\n");
      content = original = result.content.replaceAll("\r\n", "\n");
      hash = result.hash;
      readOnly = result.readOnly;
    } catch (err) {
      if (current === request) showFileError(err);
    } finally {
      if (current === request) busy = false;
    }
  }

  async function save() {
    if (!selected || busy || readOnly) return;
    busy = true;
    try {
      const saved = content;
      hash = await api.saveBrowserFile(
        selected.rootId,
        selected.path,
        usesCrLf ? saved.replaceAll("\n", "\r\n") : saved,
        hash,
        selected.absolutePath
      );
      original = saved;
      appState.addToast("success", t("files.saved"));
      appState.bumpRules();
    } catch (err) {
      if (parseError(err).code === "plan_changed")
        appState.addToast("error", t("files.changed_externally"));
      else showFileError(err);
    } finally {
      busy = false;
    }
  }

  function goHome() {
    request++;
    rootId = "";
    directory = ".";
    entries = [];
    busy = false;
    void reloadRoots();
  }

  async function reloadRoots() {
    try {
      roots = await api.listFileRoots();
    } catch (err) {
      appState.showError(err);
    }
  }

  function goBack() {
    if (!rootId) return;
    if (directory === ".") {
      goHome();
      return;
    }
    const parent = directory.includes("/") ? directory.slice(0, directory.lastIndexOf("/")) : ".";
    void openDirectory(rootId, parent);
  }

  function refresh() {
    if (rootId) void openDirectory(rootId, directory);
    else void reloadRoots();
  }

  function formatJson() {
    try {
      const newline = content.endsWith("\n") ? "\n" : "";
      content = JSON.stringify(JSON.parse(content), null, 2) + newline;
    } catch {
      appState.addToast("error", t("files.invalid_json"));
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

  function togglePreview() {
    editing = preview;
  }
</script>

<section class="view">
  <div class="files-header">
    <ViewHeader title={t("files.title")} intro={t("files.intro")} />
  </div>
  {#if loading}<Spinner size="sm" />{:else}
    <div class="workspace">
      <nav aria-label={t("files.roots")} class="file-browser">
        <div class="browser-toolbar">
          <IconButton label={t("files.back")} disabled={!rootId} onclick={goBack}
            ><ArrowLeft size={16} /></IconButton
          >
          <IconButton label={t("files.home")} disabled={!rootId} onclick={goHome}
            ><Home size={16} /></IconButton
          >
          <IconButton label={t("files.refresh")} onclick={refresh}
            ><RefreshCw size={16} /></IconButton
          >
        </div>
        <h3 title={rootId ? `${rootName}/${directory}` : t("files.roots")}>
          {rootId ? `${rootName}${directory === "." ? "" : ` / ${directory}`}` : t("files.roots")}
        </h3>
        {#if rootId}
          {#each entries as entry (entry.path)}
            <button
              type="button"
              class:active={selected?.rootId === rootId &&
                selected?.path === entry.path &&
                !entry.isDir}
              disabled={busy}
              title={entry.path}
              onclick={() =>
                entry.isDir
                  ? openDirectory(rootId, entry.path)
                  : openFile(rootId, entry.path, entry.name)}
            >
              {#if entry.isDir}<Folder size={16} />{:else}<FileText size={16} />{/if}
              <span>{entry.name}</span>
            </button>
          {/each}
          {#if entries.length === 0 && !busy}<p>{t("files.empty_folder")}</p>{/if}
        {:else}
          {#each roots as root (root.id)}
            <button
              type="button"
              disabled={busy}
              title={root.name}
              onclick={() => openDirectory(root.id)}
            >
              <Folder size={16} /><span>{root.name}</span>
            </button>
          {/each}
          {#if roots.length === 0 && !busy}<p>{t("files.select_repository")}</p>{/if}
        {/if}
      </nav>
      <div class="editor-panel">
        {#if selected}
          <div class="toolbar">
            <strong title={selected.absolutePath}>{selected.name}</strong>
            {#if readOnly}<span>{t("files.read_only")}</span>{/if}
            <div class="actions">
              {#if !readOnly && isMarkdown && !preview}
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
              {#if !readOnly && isJson}
                <IconButton label={t("files.format_json")} disabled={busy} onclick={formatJson}
                  ><Braces size={16} /></IconButton
                >
              {/if}
              <IconButton label={t("files.open_explorer")} onclick={openExplorer}
                ><FolderOpen size={16} /></IconButton
              >
              {#if isMarkdown}
                <IconButton label={t("files.preview")} ariaPressed={preview} onclick={togglePreview}
                  ><Eye size={16} /></IconButton
                >
              {/if}
              {#if !readOnly}<Button
                  size="sm"
                  variant="primary"
                  disabled={busy || !appState.fileDirty}
                  onclick={save}>{t("files.save")}</Button
                >{/if}
            </div>
          </div>
          {#if preview}
            <MarkdownPreview source={content} />
          {:else}<textarea
              bind:this={editor}
              bind:value={content}
              readonly={readOnly || busy}
              aria-label={t("files.editor_label", { name: selected.name })}
              spellcheck="true"></textarea>{/if}
        {:else}<p class="hint">
            {t(rootId ? "files.select_file" : "files.select_repository")}
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
    min-height: 0;
  }
  .files-header {
    width: 100%;
    max-width: calc(var(--content-max-width) - 2 * var(--space-8));
    margin-inline: auto;
  }
  p {
    color: var(--text-muted);
    font-size: var(--text-sm);
  }
  .workspace {
    display: grid;
    flex: 1;
    grid-template-columns: minmax(180px, 280px) minmax(0, 1fr);
    grid-template-rows: minmax(0, 1fr);
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
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  nav button {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    gap: var(--space-2);
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
  nav button :global(svg) {
    flex-shrink: 0;
  }
  nav button span {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .browser-toolbar {
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }
  .browser-toolbar :global(button) {
    flex-shrink: 0;
  }
  nav button:hover,
  nav button.active {
    background: var(--surface-sunken);
  }
  nav button:focus-visible {
    outline: 2px solid var(--ink);
  }
  .editor-panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    min-height: 0;
    --markdown-preview-min-height: 360px;
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
    flex: 1 1 0;
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
      grid-template-columns: 1fr;
      grid-template-rows: auto minmax(0, 1fr);
    }
    .file-browser {
      max-height: 240px;
    }
  }
</style>
