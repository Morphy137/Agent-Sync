<script lang="ts">
  import { untrack } from "svelte";
  import { Plus, Sparkles } from "@lucide/svelte";
  import { rules } from "../../../../wailsjs/go/models";
  import { api, type RuleScope } from "../../api";
  import { appState } from "../../stores/appState.svelte";
  import { t } from "../../i18n";
  import Modal from "../Modal.svelte";
  import Button from "../Button.svelte";
  import Input from "../Input.svelte";
  import Select from "../Select.svelte";
  import SegmentedControl from "../SegmentedControl.svelte";
  import Switch from "../Switch.svelte";
  import Badge from "./Badge.svelte";
  import SourcePicker from "./SourcePicker.svelte";
  import DestinationEditor from "./DestinationEditor.svelte";
  import CollisionList from "./CollisionList.svelte";
  import {
    AUTO_KIND,
    KINDS,
    applySuggestion,
    baseName,
    buildRulePayload,
    draftFromDestination,
    emptyDestinationDraft,
    hasErrors,
    isFolderKind,
    toLocationOptions,
    validateRuleForm,
    type ContentKind,
    type DestinationDraft,
    type LocationOption,
    type RuleForm,
    type RuleFormErrors,
  } from "./ruleUi";

  const hasRoot = $derived(Boolean(appState.config?.rootPath));

  let form = $state<RuleForm>(blankForm("repo"));
  let errors = $state<RuleFormErrors>({ byDestination: {} });
  let showErrors = $state(false);
  let locations = $state<LocationOption[]>([]);
  let detectedKind = $state("");
  let suggestingKeys = $state<string[]>([]);
  let saving = $state(false);
  let savedCollisions = $state<rules.Collision[]>([]);
  let knownRules = $state<rules.Rule[]>([]);

  const isEdit = $derived(Boolean(appState.ruleEditor.rule?.id));
  const title = $derived(isEdit ? t("rules.editor.title_edit") : t("rules.editor.title_new"));
  const scopeOptions = $derived([
    { value: "repo", label: t("rules.scope.repo") },
    { value: "local", label: t("rules.scope.local") },
  ]);
  const kindOptions = $derived(KINDS.map((k) => ({ value: k, label: t(`rules.kind.${k}`) })));

  function blankForm(scope: RuleScope): RuleForm {
    return {
      id: "",
      name: "",
      enabled: true,
      scope,
      source: null,
      kind: AUTO_KIND,
      sourceIsDir: false,
      excludeText: "",
      destinations: [emptyDestinationDraft()],
    };
  }

  function formFromRule(rule: rules.Rule, scope: RuleScope): RuleForm {
    return {
      id: rule.id ?? "",
      name: rule.name ?? "",
      enabled: rule.enabled ?? true,
      scope,
      source: rule.source ? rules.Source.createFrom(rule.source) : null,
      kind: (KINDS as readonly string[]).includes(rule.kind)
        ? (rule.kind as ContentKind)
        : AUTO_KIND,
      sourceIsDir: false,
      excludeText: (rule.exclude ?? []).join("\n"),
      destinations: (rule.destinations ?? []).map(draftFromDestination),
    };
  }

  // Reset the form every time the editor opens.
  $effect(() => {
    const editor = appState.ruleEditor;
    if (!editor.open) return;
    untrack(() => {
      const initial = editor.rule
        ? formFromRule(editor.rule, editor.scope)
        : { ...blankForm(editor.scope), source: editor.presetSource };
      if (initial.source?.origin === "subscription" && !editor.rule) initial.scope = "local";
      form = initial;
      errors = { byDestination: {} };
      showErrors = false;
      detectedKind = "";
      savedCollisions = [];
      void loadLocations();
      if (form.source) void detectSource(form.source, !editor.rule);
    });
  });

  // Live validation once the user tried to save.
  $effect(() => {
    if (showErrors) errors = validateRuleForm(form, hasRoot);
  });

  async function loadLocations() {
    try {
      locations = toLocationOptions(await api.listLocations());
    } catch (err) {
      appState.showError(err);
    }
  }

  async function detectSource(source: rules.Source, isNewSource: boolean) {
    try {
      const info = await api.detectSource(source);
      detectedKind = info.kind;
      form.sourceIsDir = info.isDir;
      if (isNewSource && !form.name.trim()) {
        form.name = info.baseName || info.name || baseName(source.path);
      }
    } catch (err) {
      detectedKind = "";
      appState.showError(err);
    }
  }

  async function handleSourceChange(source: rules.Source) {
    form.source = source;
    if (source.origin === "local" || source.origin === "subscription") form.scope = "local";
    await detectSource(source, true);
    // Re-suggest destinations the user has not customized yet.
    for (const draft of form.destinations) {
      if (draft.location && !draft.touched) void suggestFor(draft.key, draft.location);
    }
  }

  function updateDraft(key: string, patch: Partial<DestinationDraft>) {
    form.destinations = form.destinations.map((d) => (d.key === key ? { ...d, ...patch } : d));
  }

  async function suggestFor(key: string, alias: string) {
    if (!form.source) return;
    suggestingKeys = [...suggestingKeys, key];
    try {
      const suggestion = await api.suggestPlacement(form.source, alias);
      form.destinations = form.destinations.map((d) =>
        d.key === key && !d.touched ? applySuggestion(d, suggestion) : d
      );
    } catch (err) {
      appState.showError(err);
    } finally {
      suggestingKeys = suggestingKeys.filter((k) => k !== key);
    }
  }

  function handleLocation(key: string, alias: string) {
    updateDraft(key, { location: alias, touched: false });
    void suggestFor(key, alias);
  }

  async function handleLocationCreated(key: string, alias: string) {
    await loadLocations();
    handleLocation(key, alias);
  }

  function handleDraftChange(key: string, patch: Partial<DestinationDraft>) {
    const editsPlacement = "path" in patch || "mode" in patch || "transform" in patch;
    updateDraft(key, editsPlacement ? { ...patch, touched: true } : patch);
  }

  function addDestination() {
    form.destinations = [...form.destinations, emptyDestinationDraft()];
  }

  function removeDestination(key: string) {
    form.destinations = form.destinations.filter((d) => d.key !== key);
  }

  function close() {
    appState.closeRuleEditor();
  }

  async function save() {
    showErrors = true;
    errors = validateRuleForm(form, hasRoot);
    if (hasErrors(errors) || saving) return;
    saving = true;
    try {
      const result = await api.saveRule(buildRulePayload(form), form.scope);
      appState.bumpRules();
      if (result.collisions && result.collisions.length > 0) {
        const report = await api.getRulesReport();
        knownRules = report.rules.map((r) => r.rule);
        savedCollisions = result.collisions;
        return;
      }
      appState.addToast("success", t("rules.editor.saved"));
      close();
    } catch (err) {
      appState.showError(err);
    } finally {
      saving = false;
    }
  }

  function acknowledgeCollisions() {
    savedCollisions = [];
    close();
  }
</script>

<Modal open={appState.ruleEditor.open} size="lg" {title} onclose={close}>
  {#if savedCollisions.length > 0}
    <div class="collision-warning" role="alert">
      <h3 class="section-title">{t("rules.editor.collisions_title")}</h3>
      <p class="hint">{t("rules.editor.collisions_desc")}</p>
      <CollisionList collisions={savedCollisions} {knownRules} />
      <footer class="editor-footer">
        <Button variant="primary" size="sm" onclick={acknowledgeCollisions}>
          {t("rules.editor.collisions_ack")}
        </Button>
      </footer>
    </div>
  {:else}
    <form
      class="editor"
      onsubmit={(e) => {
        e.preventDefault();
        void save();
      }}
    >
      <div class="row">
        <div class="grow">
          <Input
            label={t("rules.editor.name_label")}
            value={form.name}
            placeholder={t("rules.editor.name_placeholder")}
            oninput={(name) => (form.name = name)}
          />
          {#if errors.name}<p class="error">{t(errors.name)}</p>{/if}
        </div>
        <div class="enabled">
          <Switch
            checked={form.enabled}
            label={t("rules.editor.enabled_label")}
            onchange={(enabled) => (form.enabled = enabled)}
          />
        </div>
      </div>

      <section class="section">
        <SegmentedControl
          label={t("rules.editor.scope_label")}
          options={scopeOptions}
          value={form.scope}
          size="sm"
          onchange={(scope) => (form.scope = scope as RuleScope)}
        />
        <p class="hint">
          {form.scope === "repo" ? t("rules.scope.repo_hint") : t("rules.scope.local_hint")}
        </p>
        {#if errors.scope}<p class="error">{t(errors.scope)}</p>{/if}
      </section>

      <section class="section">
        <h3 class="section-title">{t("rules.editor.source_title")}</h3>
        <SourcePicker value={form.source} {hasRoot} onchange={handleSourceChange} />
        {#if errors.source}<p class="error">{t(errors.source)}</p>{/if}
        {#if form.source}
          <div class="detected">
            {#if detectedKind}
              <Badge tone="success">
                <Sparkles size={12} aria-hidden="true" />
                {t("rules.editor.detected_kind", { kind: t(`rules.kind.${detectedKind}`) })}
              </Badge>
            {/if}
            <div class="kind-select">
              <Select
                label={t("rules.editor.kind_label")}
                value={form.kind}
                options={kindOptions}
                onchange={(kind) => (form.kind = kind)}
              />
            </div>
          </div>
        {/if}
        {#if form.sourceIsDir || isFolderKind(form.kind)}
          <label class="field-label" for="rule-exclude">{t("rules.editor.exclude_label")}</label>
          <textarea
            id="rule-exclude"
            class="exclude"
            rows="3"
            placeholder={t("rules.editor.exclude_placeholder")}
            value={form.excludeText}
            oninput={(e) => (form.excludeText = (e.target as HTMLTextAreaElement).value)}
          ></textarea>
          <p class="hint">{t("rules.editor.exclude_hint")}</p>
        {/if}
      </section>

      <section class="section">
        <div class="section-head">
          <h3 class="section-title">{t("rules.editor.destinations_title")}</h3>
          <Button variant="ghost" size="sm" onclick={addDestination}>
            <Plus size={14} />
            <span>{t("rules.editor.add_destination")}</span>
          </Button>
        </div>
        <p class="hint">{t("rules.editor.destinations_hint")}</p>
        {#if errors.destinations}<p class="error">{t(errors.destinations)}</p>{/if}
        <div class="destinations">
          {#each form.destinations as draft, index (draft.key)}
            <DestinationEditor
              {draft}
              {index}
              {locations}
              errors={errors.byDestination[draft.key]}
              sourceIsDir={form.sourceIsDir}
              suggesting={suggestingKeys.includes(draft.key)}
              onchange={(patch) => handleDraftChange(draft.key, patch)}
              onlocation={(alias) => handleLocation(draft.key, alias)}
              onlocationcreated={(alias) => handleLocationCreated(draft.key, alias)}
              onremove={() => removeDestination(draft.key)}
            />
          {/each}
        </div>
      </section>

      <footer class="editor-footer">
        <Button variant="ghost" size="sm" onclick={close}>{t("common.cancel")}</Button>
        <Button variant="primary" size="sm" type="submit" loading={saving}>
          {t("rules.editor.save")}
        </Button>
      </footer>
    </form>
  {/if}
</Modal>

<style>
  .editor,
  .collision-warning {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  .row {
    display: flex;
    align-items: flex-end;
    gap: var(--space-4);
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  .enabled {
    padding-bottom: var(--space-2);
  }

  .section {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .section-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .section-title {
    margin: 0;
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .hint {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .error {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--dot-conflict);
  }

  .detected {
    display: flex;
    align-items: flex-end;
    flex-wrap: wrap;
    gap: var(--space-3);
  }

  .kind-select {
    min-width: 200px;
  }

  .field-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .exclude {
    resize: vertical;
    padding: var(--space-2) var(--space-3);
    border: none;
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--ink);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    outline: none;
  }

  .exclude:focus-visible {
    box-shadow: var(--focus-ring);
  }

  .destinations {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .editor-footer {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    padding-top: var(--space-2);
    border-top: 1px solid var(--hairline);
  }
</style>
