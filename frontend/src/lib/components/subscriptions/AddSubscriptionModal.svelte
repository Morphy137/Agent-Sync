<script lang="ts">
  import { untrack } from "svelte";
  import type { config } from "../../../../wailsjs/go/models";
  import { api } from "../../api";
  import { t } from "../../i18n";
  import { appState } from "../../stores/appState.svelte";
  import Modal from "../Modal.svelte";
  import Button from "../Button.svelte";
  import Input from "../Input.svelte";
  import Spinner from "../Spinner.svelte";
  import { isValidRemoteUrl, repoNameFromUrl } from "../remote/remoteUrl";
  import { reportError } from "../remote/reportError";

  interface Props {
    open: boolean;
    onclose: () => void;
    onadded: (subscription: config.Subscription) => void;
  }

  let { open, onclose, onadded }: Props = $props();

  let url = $state("");
  let name = $state("");
  let adding = $state(false);

  const trimmedUrl = $derived(url.trim());
  const urlInvalid = $derived(trimmedUrl.length > 0 && !isValidRemoteUrl(trimmedUrl));
  const canSubmit = $derived(trimmedUrl.length > 0 && !urlInvalid && !adding);
  const suggestedName = $derived(urlInvalid ? "" : repoNameFromUrl(trimmedUrl));

  $effect(() => {
    if (!open) return;
    untrack(() => {
      url = "";
      name = "";
      adding = false;
    });
  });

  function handleClose() {
    if (adding) return;
    onclose();
  }

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault();
    if (!canSubmit) return;
    adding = true;
    try {
      const created = await api.addSubscription(trimmedUrl, name.trim());
      appState.addToast("success", t("subscriptions.toasts.added", { name: created.name }));
      onadded(created);
    } catch (err) {
      reportError(err);
    } finally {
      adding = false;
    }
  }
</script>

<Modal {open} title={t("subscriptions.add.title")} onclose={handleClose}>
  <form class="add-form" onsubmit={handleSubmit} novalidate>
    <p class="intro">{t("subscriptions.add.intro")}</p>

    <div class="field">
      <Input
        label={t("subscriptions.add.url_label")}
        placeholder={t("remote.url.placeholder")}
        value={url}
        disabled={adding}
        oninput={(value) => (url = value)}
      />
      <p class="hint" class:hint-error={urlInvalid} aria-live="polite">
        {urlInvalid ? t("remote.url.invalid") : t("remote.url.hint")}
      </p>
    </div>

    <div class="field">
      <Input
        label={t("subscriptions.add.name_label")}
        placeholder={suggestedName || t("subscriptions.add.name_placeholder")}
        value={name}
        disabled={adding}
        oninput={(value) => (name = value)}
      />
      <p class="hint">{t("subscriptions.add.name_hint")}</p>
    </div>

    {#if adding}
      <div class="progress" role="status">
        <Spinner size="sm" />
        <span>{t("subscriptions.add.cloning")}</span>
      </div>
    {/if}

    <div class="actions">
      <Button variant="secondary" disabled={adding} onclick={handleClose}>
        {t("common.cancel")}
      </Button>
      <Button type="submit" variant="primary" loading={adding} disabled={!canSubmit}>
        {adding ? t("subscriptions.add.submitting") : t("subscriptions.add.submit")}
      </Button>
    </div>
  </form>
</Modal>

<style>
  .add-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .intro {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
    line-height: 1.55;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .hint {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .hint-error {
    color: var(--dot-conflict);
  }

  .progress {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    font-size: var(--text-xs);
    color: var(--ink);
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-3);
  }
</style>
