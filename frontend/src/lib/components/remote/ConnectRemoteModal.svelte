<script lang="ts">
  import { untrack } from "svelte";
  import {
    CircleCheck,
    FolderCheck,
    GitBranch,
    History,
    ListChecks,
    UserRound,
  } from "@lucide/svelte";
  import type { gitops } from "../../../../wailsjs/go/models";
  import { api } from "../../api";
  import { t } from "../../i18n";
  import { appState } from "../../stores/appState.svelte";
  import Modal from "../Modal.svelte";
  import Button from "../Button.svelte";
  import Input from "../Input.svelte";
  import { isValidRemoteUrl } from "./remoteUrl";
  import { reportError } from "./reportError";
  import { changeStatusLabel, classifyGitStatus } from "./repoChanges";

  interface Props {
    open: boolean;
    onclose: () => void;
    /** Opens the change review panel for the local differences left after connecting. */
    onreview: () => void;
  }

  let { open, onclose, onreview }: Props = $props();

  const IDENTITY_MISSING_CODE = "git_identity_missing";

  let url = $state("");
  let connecting = $state(false);
  let result = $state<gitops.ConnectRemoteResult | null>(null);
  let identityRetry = $state(false);

  const trimmedUrl = $derived(url.trim());
  const urlInvalid = $derived(trimmedUrl.length > 0 && !isValidRemoteUrl(trimmedUrl));
  const canSubmit = $derived(trimmedUrl.length > 0 && !urlInvalid && !connecting);
  const folderIsRepo = $derived(appState.gitInfo?.isRepo ?? false);
  const localChanges = $derived(result?.localChanges ?? []);

  $effect(() => {
    if (!open) return;
    untrack(() => {
      url = appState.gitInfo?.remoteUrl ?? "";
      result = null;
      identityRetry = false;
      connecting = false;
    });
  });

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault();
    if (!canSubmit) return;
    connecting = true;
    identityRetry = false;
    try {
      result = await api.connectRemote(trimmedUrl);
      appState.addToast("success", t("remote.connect.toast_success"));
      await appState.refreshAll();
    } catch (err) {
      identityRetry = reportError(err) === IDENTITY_MISSING_CODE;
    } finally {
      connecting = false;
    }
  }
</script>

<Modal {open} title={t("remote.connect.title")} {onclose}>
  {#if result}
    <div class="result" aria-live="polite">
      <div class="result-head">
        <span class="result-icon"><CircleCheck size={20} /></span>
        <div class="result-head-text">
          <p class="result-title">{t("remote.connect.result_title")}</p>
          <p class="muted">{t("remote.connect.result_branch", { branch: result.branch })}</p>
        </div>
      </div>

      <ul class="facts">
        <li class="fact">
          <GitBranch size={15} />
          <span>
            {result.remoteHadContent
              ? t("remote.connect.remote_had_content")
              : t("remote.connect.remote_was_empty")}
          </span>
        </li>
        {#if result.backupBranch}
          <li class="fact">
            <History size={15} />
            <div class="fact-body">
              <span>{t("remote.connect.backup_branch_title")}</span>
              <code class="mono-chip">{result.backupBranch}</code>
              <span class="muted">{t("remote.connect.backup_branch_hint")}</span>
            </div>
          </li>
        {/if}
      </ul>

      {#if localChanges.length > 0}
        <section class="changes" aria-labelledby="connect-local-changes-title">
          <h3 id="connect-local-changes-title" class="changes-title">
            {t("remote.connect.local_changes_title", { count: localChanges.length })}
          </h3>
          <p class="muted">{t("remote.connect.local_changes_hint")}</p>
          <ul class="change-list">
            {#each localChanges as change (change.path)}
              {@const status = classifyGitStatus(change.status)}
              <li class="change-row">
                <span class="status-tag status-{status}">{changeStatusLabel(status)}</span>
                <span class="change-path" title={change.path}>{change.path}</span>
              </li>
            {/each}
          </ul>
        </section>
      {:else}
        <p class="muted">{t("remote.connect.no_local_changes")}</p>
      {/if}

      <div class="actions">
        <Button variant={localChanges.length > 0 ? "secondary" : "primary"} onclick={onclose}>
          {t("remote.connect.done")}
        </Button>
        {#if localChanges.length > 0}
          <Button variant="primary" onclick={onreview}>
            <ListChecks size={16} />
            <span>{t("remote.connect.review_changes")}</span>
          </Button>
        {/if}
      </div>
    </div>
  {:else}
    <form class="connect-form" onsubmit={handleSubmit} novalidate>
      <p class="intro">
        {folderIsRepo ? t("remote.connect.intro_repo") : t("remote.connect.intro_not_repo")}
      </p>

      <ul class="guarantees">
        <li>
          <FolderCheck size={15} />
          <span>{t("remote.connect.point_keep_files")}</span>
        </li>
        <li>
          <ListChecks size={15} />
          <span>{t("remote.connect.point_review")}</span>
        </li>
        <li>
          <History size={15} />
          <span>{t("remote.connect.point_backup_branch")}</span>
        </li>
      </ul>

      <Input
        label={t("remote.url.label")}
        placeholder={t("remote.url.placeholder")}
        value={url}
        disabled={connecting}
        oninput={(value) => (url = value)}
      />
      <p class="hint" class:hint-error={urlInvalid} aria-live="polite">
        {urlInvalid ? t("remote.url.invalid") : t("remote.url.hint")}
      </p>

      {#if identityRetry}
        <p class="notice" role="status">
          <UserRound size={15} />
          <span>{t("remote.connect.identity_retry")}</span>
        </p>
      {/if}

      {#if connecting}
        <p class="muted" aria-live="polite">{t("remote.connect.connecting_hint")}</p>
      {/if}

      <div class="actions">
        <Button variant="secondary" onclick={onclose}>{t("common.cancel")}</Button>
        <Button type="submit" variant="primary" loading={connecting} disabled={!canSubmit}>
          {connecting ? t("remote.connect.connecting") : t("remote.connect.submit")}
        </Button>
      </div>
    </form>
  {/if}
</Modal>

<style>
  .connect-form,
  .result {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .intro {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--ink);
    line-height: 1.55;
  }

  .guarantees,
  .facts {
    list-style: none;
    margin: 0;
    padding: var(--space-3) var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-md);
  }

  .guarantees li,
  .fact {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.5;
  }

  .guarantees li :global(svg),
  .fact :global(svg) {
    flex-shrink: 0;
    margin-top: 1px;
    color: var(--ink);
  }

  .fact-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    min-width: 0;
  }

  .mono-chip {
    align-self: flex-start;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    border-radius: var(--radius-xs);
    padding: 1px var(--space-2);
    word-break: break-all;
  }

  .hint {
    margin: calc(-1 * var(--space-2)) 0 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .hint-error {
    color: var(--dot-conflict);
  }

  .notice {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    margin: 0;
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    font-size: var(--text-xs);
    color: var(--ink);
  }

  .muted {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.5;
  }

  .result-head {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .result-icon {
    width: 36px;
    height: 36px;
    flex-shrink: 0;
    border-radius: var(--radius-md);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--dot-ok);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .result-head-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .result-title {
    margin: 0;
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .changes {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .changes-title {
    margin: 0;
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .change-list {
    list-style: none;
    margin: 0;
    padding: var(--space-2) var(--space-3);
    max-height: 200px;
    overflow-y: auto;
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-md);
  }

  .change-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: 28px;
  }

  .status-tag {
    flex-shrink: 0;
    min-width: 72px;
    text-align: center;
    font-size: 11px;
    font-weight: var(--weight-medium);
    padding: 1px var(--space-2);
    border-radius: var(--radius-xs);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    color: var(--text-muted);
  }

  .status-added,
  .status-untracked {
    color: var(--dot-ok);
  }

  .status-modified,
  .status-renamed {
    color: var(--dot-up);
  }

  .status-deleted {
    color: var(--dot-conflict);
  }

  .change-path {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    flex-wrap: wrap;
    gap: var(--space-3);
  }
</style>
