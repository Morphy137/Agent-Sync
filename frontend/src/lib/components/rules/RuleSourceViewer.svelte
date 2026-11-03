<script lang="ts">
  import { untrack } from "svelte";
  import type { service } from "../../../../wailsjs/go/models";
  import { api, parseError } from "../../api";
  import { t } from "../../i18n";
  import { supportsPreview } from "../../fileKinds";
  import MarkdownPreview from "../MarkdownPreview.svelte";
  import Modal from "../Modal.svelte";
  import Spinner from "../Spinner.svelte";
  import { defaultViewerPath } from "./ruleUi";

  /** Strictly read-only window onto the source file(s) of a rule; nothing here can edit or save. */
  interface Props {
    open: boolean;
    ruleId: string;
    ruleName: string;
    /** Source-relative path to select when the viewer opens. */
    initialPath?: string;
    onclose: () => void;
  }

  type Load<T> =
    { state: "loading" } | { state: "ready"; value: T } | { state: "error"; messageKey: string };

  /** Backend error reasons and codes mapped to what the viewer tells the user. */
  const FILE_ERROR_KEYS: Record<string, string> = {
    file_too_large: "rules.viewer.file_too_large",
    file_not_text: "rules.viewer.file_not_text",
    source_not_found: "rules.viewer.file_missing",
  };
  const FILE_ERROR_FALLBACK = "rules.viewer.file_error";
  /** The source folder itself cannot be listed (rule removed, source deleted). */
  const SOURCE_ERROR_KEY = "rules.viewer.source_unavailable";
  const CURRENT_ITEM = "true";

  let { open, ruleId, ruleName, initialPath = "", onclose }: Props = $props();

  let files = $state<Load<service.RuleSourceFile[]>>({ state: "loading" });
  let selectedPath = $state("");
  let content = $state<Load<string>>({ state: "loading" });
  /** Responses of superseded requests are dropped, so quick switching never shows stale content. */
  let listRequest = 0;
  let fileRequest = 0;

  const fileList = $derived(files.state === "ready" ? files.value : []);
  const selectedFile = $derived(fileList.find((file) => file.path === selectedPath));
  const showSidebar = $derived(fileList.length > 1);
  const selectedLabel = $derived(
    selectedFile ? (selectedFile.path === "." ? selectedFile.name : selectedFile.path) : ""
  );

  $effect(() => {
    if (!open) {
      listRequest++;
      fileRequest++;
      return;
    }
    const id = ruleId;
    const requested = initialPath;
    untrack(() => void loadFiles(id, requested));
  });

  async function loadFiles(id: string, requested: string) {
    const current = ++listRequest;
    fileRequest++;
    files = { state: "loading" };
    selectedPath = "";
    content = { state: "loading" };
    try {
      const list = (await api.listRuleSourceFiles(id)) ?? [];
      if (current !== listRequest) return;
      files = { state: "ready", value: list };
      const first = defaultViewerPath(list, requested);
      if (first) void selectFile(id, first);
    } catch {
      if (current === listRequest) files = { state: "error", messageKey: SOURCE_ERROR_KEY };
    }
  }

  async function selectFile(id: string, path: string) {
    const current = ++fileRequest;
    selectedPath = path;
    content = { state: "loading" };
    try {
      const result = await api.readRuleSourceFile(id, path);
      if (current === fileRequest) content = { state: "ready", value: result.content };
    } catch (err) {
      if (current !== fileRequest) return;
      const { code, params } = parseError(err);
      const messageKey =
        FILE_ERROR_KEYS[params?.reason ?? ""] ?? FILE_ERROR_KEYS[code] ?? FILE_ERROR_FALLBACK;
      content = { state: "error", messageKey };
    }
  }
</script>

<Modal {open} size="lg" title={t("rules.viewer.title", { name: ruleName })} {onclose}>
  <div class="viewer" class:with-sidebar={showSidebar}>
    {#if files.state === "loading"}
      <div class="centered"><Spinner size="sm" /></div>
    {:else if files.state === "error"}
      <p class="message" role="alert">{t(files.messageKey)}</p>
    {:else if fileList.length === 0}
      <p class="message">{t("rules.viewer.empty")}</p>
    {:else}
      {#if showSidebar}
        <nav class="sidebar" aria-label={t("rules.viewer.files_label")}>
          <ul>
            {#each fileList as file (file.path)}
              <li>
                <button
                  type="button"
                  class="file-item"
                  class:active={file.path === selectedPath}
                  aria-current={file.path === selectedPath ? CURRENT_ITEM : undefined}
                  title={file.path}
                  onclick={() => selectFile(ruleId, file.path)}
                >
                  {file.path}
                </button>
              </li>
            {/each}
          </ul>
        </nav>
      {/if}
      <section class="pane" aria-label={t("rules.viewer.content_label", { name: selectedLabel })}>
        <header class="pane-head">
          <strong title={selectedFile?.absolutePath}>{selectedLabel}</strong>
          <span class="read-only">{t("files.read_only")}</span>
        </header>
        {#if content.state === "loading"}
          <div class="centered"><Spinner size="sm" /></div>
        {:else if content.state === "error"}
          <p class="message" role="alert">{t(content.messageKey)}</p>
        {:else if selectedFile && supportsPreview(selectedFile.name)}
          <MarkdownPreview source={content.value} />
        {:else}
          <!-- Focusable so keyboard users can scroll long files. -->
          <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
          <pre class="plain" tabindex="0">{content.value}</pre>
        {/if}
      </section>
    {/if}
  </div>
</Modal>

<style>
  .viewer {
    display: flex;
    gap: var(--space-3);
    height: 100%;
    min-height: 0;
  }

  .centered {
    display: flex;
    flex: 1;
    align-items: center;
    justify-content: center;
  }

  .message {
    margin: auto;
    color: var(--text-muted);
    font-size: var(--text-sm);
    text-align: center;
  }

  /* The file list is narrower than the content and scrolls on its own, so it fits small windows. */
  .sidebar {
    flex: 0 0 clamp(140px, 28%, 240px);
    min-width: 0;
    overflow-y: auto;
    padding: var(--space-1);
    background: var(--surface-sunken);
    border-radius: var(--radius-md);
  }

  .sidebar ul {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .file-item {
    width: 100%;
    padding: var(--space-2);
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--ink);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    text-align: left;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    cursor: pointer;
  }

  .file-item:hover,
  .file-item.active {
    background: var(--surface);
    box-shadow: var(--elev-1);
  }

  .file-item:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .pane {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
    min-height: 0;
  }

  .pane-head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }

  .pane-head strong {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-sm);
    color: var(--ink);
  }

  .read-only {
    margin-left: auto;
    flex-shrink: 0;
    color: var(--text-muted);
    font-size: var(--text-xs);
  }

  .plain {
    flex: 1 1 auto;
    min-height: 0;
    margin: 0;
    overflow: auto;
    padding: var(--space-3);
    background: var(--surface-sunken);
    border-radius: var(--radius-md);
    color: var(--ink);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    line-height: 1.6;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    user-select: text;
  }

  .plain:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }
</style>
