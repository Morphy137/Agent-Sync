<script lang="ts">
  import { t } from "../i18n";
  import Modal from "./Modal.svelte";
  import Button from "./Button.svelte";
  import Input from "./Input.svelte";

  interface Props {
    open: boolean;
    initialName?: string;
    initialEmail?: string;
    onclose: () => void;
    onsubmit: (name: string, email: string) => void;
  }

  let { open, initialName = "", initialEmail = "", onclose, onsubmit }: Props = $props();

  let userName = $state("");
  let userEmail = $state("");

  $effect(() => {
    if (open) {
      userName = initialName;
      userEmail = initialEmail;
    }
  });

  function handleSubmit() {
    if (userEmail.trim()) {
      onsubmit(userName.trim(), userEmail.trim());
    }
  }
</script>

<Modal {open} title={t("dialogs.git_identity_title")} {onclose}>
  <div class="identity-form">
    <p class="desc">{t("dialogs.git_identity_description")}</p>

    <Input
      label={t("settings.git_name_label")}
      placeholder={t("settings.git_name_placeholder")}
      value={userName}
      oninput={(val) => (userName = val)}
    />

    <Input
      label={t("settings.git_email_label")}
      placeholder={t("settings.git_email_placeholder")}
      value={userEmail}
      oninput={(val) => (userEmail = val)}
    />

    <div class="form-actions">
      <Button variant="secondary" onclick={onclose}>
        {t("common.cancel")}
      </Button>
      <Button variant="primary" disabled={!userEmail.trim()} onclick={handleSubmit}>
        {t("dialogs.git_identity_submit")}
      </Button>
    </div>
  </div>
</Modal>

<style>
  .identity-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .desc {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
    line-height: 1.5;
  }

  .form-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-3);
    margin-top: var(--space-2);
  }
</style>
