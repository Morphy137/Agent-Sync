<script lang="ts">
  import { t } from "../i18n";
  import Modal from "./Modal.svelte";
  import Button from "./Button.svelte";
  import Input from "./Input.svelte";

  interface Props {
    open: boolean;
    onclose: () => void;
    onsubmit: (url: string) => void;
  }

  let { open, onclose, onsubmit }: Props = $props();

  let remoteUrl = $state("");

  function handleSubmit() {
    if (remoteUrl.trim()) {
      onsubmit(remoteUrl.trim());
    }
  }
</script>

<Modal {open} title={t("dialogs.add_remote_title")} {onclose}>
  <div class="add-remote-form">
    <Input
      label={t("dialogs.add_remote_input_label")}
      placeholder={t("dialogs.add_remote_placeholder")}
      value={remoteUrl}
      oninput={(val) => (remoteUrl = val)}
    />

    <div class="form-actions">
      <Button variant="secondary" onclick={onclose}>
        {t("common.cancel")}
      </Button>
      <Button variant="primary" disabled={!remoteUrl.trim()} onclick={handleSubmit}>
        {t("dialogs.add_remote_submit")}
      </Button>
    </div>
  </div>
</Modal>

<style>
  .add-remote-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  .form-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-3);
  }
</style>
