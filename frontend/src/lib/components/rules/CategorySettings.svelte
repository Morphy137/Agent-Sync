<script lang="ts">
  import { Check, Pencil, Trash2 } from "@lucide/svelte";
  import { rules } from "../../../../wailsjs/go/models";
  import { api, type RuleScope } from "../../api";
  import { t } from "../../i18n";
  import { appState } from "../../stores/appState.svelte";
  import Button from "../Button.svelte";
  import Input from "../Input.svelte";
  import IconButton from "../IconButton.svelte";
  import Select from "../Select.svelte";
  import Modal from "../Modal.svelte";
  import "./categoryColors.css";
  import {
    CATEGORY_COLORS,
    CATEGORY_ICONS,
    categoryColor,
    categoryIcon,
    categoryIconId,
    categoryLabel,
  } from "./categories";

  interface Props {
    open: boolean;
  }
  let { open }: Props = $props();
  let categories = $state<rules.Category[]>([]);
  let editing = $state<rules.Category | null>(null);
  let removing = $state<rules.Category | null>(null);
  let saving = $state(false);
  const builtinIds = ["skill", "mcp", "instructions"];

  function edit(category: rules.Category) {
    editing = rules.Category.createFrom({
      ...category,
      icon: categoryIconId(category.icon, category.id),
      scope: category.scope === "builtin" ? "local" : category.scope,
    });
  }

  $effect(() => {
    if (open) void load();
  });

  async function load() {
    try {
      categories = (await api.getRuleCategories()) ?? [];
    } catch (err) {
      appState.showError(err);
    }
  }

  function add() {
    editing = rules.Category.createFrom({
      id: "",
      name: "",
      color: "blue",
      icon: "folder",
      scope: "local",
    });
  }

  async function save() {
    if (!editing || (!editing.name.trim() && !builtinIds.includes(editing.id)) || saving) return;
    saving = true;
    try {
      await api.saveRuleCategory(
        rules.Category.createFrom({ ...editing, name: editing.name.trim() }),
        editing.scope as RuleScope
      );
      editing = null;
      await load();
      appState.bumpRules();
    } catch (err) {
      appState.showError(err);
    } finally {
      saving = false;
    }
  }

  async function remove() {
    if (!removing || saving) return;
    saving = true;
    try {
      await api.deleteRuleCategory(
        removing.id,
        (removing.scope === "builtin" ? "local" : removing.scope) as RuleScope
      );
      removing = null;
      await load();
      appState.bumpRules();
    } catch (err) {
      appState.showError(err);
    } finally {
      saving = false;
    }
  }
</script>

<div class="categories">
  <p class="hint">{t("rules.categories.description")}</p>
  <ul class="list">
    {#each categories.filter((category) => category.id !== "uncategorized") as category (category.id)}
      {@const Icon = categoryIcon(category.icon, category.id)}
      <li class="category-row">
        <span class="category-icon" style:color={categoryColor(category.color)} aria-hidden="true"
          ><Icon size={16} /></span
        >
        <span class="name">{categoryLabel(category)}</span>
        <IconButton
          label={t("rules.categories.edit") + ": " + categoryLabel(category)}
          onclick={() => edit(category)}
        >
          <Pencil size={16} />
        </IconButton>
        <IconButton
          label={t("rules.categories.delete") + ": " + categoryLabel(category)}
          onclick={() => (removing = category)}
        >
          <Trash2 size={16} />
        </IconButton>
      </li>
    {/each}
  </ul>
  <Button size="sm" onclick={add}>{t("rules.categories.add")}</Button>
</div>

<Modal
  open={Boolean(editing)}
  title={t(editing?.id ? "rules.categories.edit" : "rules.categories.add")}
  onclose={() => (editing = null)}
>
  {#if editing}
    {@const PreviewIcon = categoryIcon(editing.icon, editing.id)}
    <form
      class="form"
      onsubmit={(event) => {
        event.preventDefault();
        void save();
      }}
    >
      <Input
        label={t("rules.categories.name")}
        value={editing.name}
        placeholder={builtinIds.includes(editing.id) ? t(`rules.categories.${editing.id}`) : ""}
        oninput={(value) => {
          if (editing) editing = rules.Category.createFrom({ ...editing, name: value });
        }}
      />
      <div class="choice-group" role="group" aria-label={t("rules.categories.color")}>
        <span class="preview-label">{t("rules.categories.color")}</span>
        <div class="choices">
          {#each CATEGORY_COLORS as color (color)}
            <button
              type="button"
              class="choice color-choice"
              class:selected={editing.color === color}
              aria-label={t(`rules.categories.colors.${color}`)}
              title={t(`rules.categories.colors.${color}`)}
              aria-pressed={editing.color === color}
              style:--choice-color={categoryColor(color)}
              onclick={() => {
                if (editing) editing = rules.Category.createFrom({ ...editing, color });
              }}
            >
              <span class="swatch"
                >{#if editing.color === color}<Check size={14} />{/if}</span
              >
            </button>
          {/each}
        </div>
      </div>
      <div class="choice-group" role="group" aria-label={t("rules.categories.icon")}>
        <span class="preview-label">{t("rules.categories.icon")}</span>
        <div class="choices">
          {#each Object.entries(CATEGORY_ICONS) as [icon, Icon] (icon)}
            <button
              type="button"
              class="choice"
              class:selected={categoryIconId(editing.icon, editing.id) === icon}
              aria-label={t(`rules.categories.icons.${icon}`)}
              title={t(`rules.categories.icons.${icon}`)}
              aria-pressed={categoryIconId(editing.icon, editing.id) === icon}
              onclick={() => {
                if (editing) editing = rules.Category.createFrom({ ...editing, icon });
              }}><Icon size={18} /></button
            >
          {/each}
        </div>
      </div>
      <div class="preview-group">
        <span class="preview-label">{t("rules.categories.preview")}</span>
        <div class="preview-section" style:--category-color={categoryColor(editing.color)}>
          <div class="preview">
            <span class="category-icon" aria-hidden="true"><PreviewIcon size={16} /></span>
            <span class="preview-name"
              >{categoryLabel(editing) || t("rules.categories.preview_name")}</span
            >
          </div>
          <div class="preview-rule category-rule-surface">{t("rules.categories.preview_rule")}</div>
        </div>
      </div>
      {#if !editing.id || builtinIds.includes(editing.id)}
        <Select
          label={t("rules.categories.scope")}
          value={editing.scope ?? "local"}
          options={[
            ...(appState.config?.rootPath ? [{ value: "repo", label: t("rules.scope.repo") }] : []),
            { value: "local", label: t("rules.scope.local") },
          ]}
          onchange={(value) => {
            if (editing) editing = rules.Category.createFrom({ ...editing, scope: value });
          }}
        />
      {/if}
      <div class="actions">
        <Button size="sm" onclick={() => (editing = null)}>{t("common.cancel")}</Button>
        <Button
          size="sm"
          variant="primary"
          type="submit"
          disabled={!editing.name.trim() && !builtinIds.includes(editing.id)}
          loading={saving}>{t("rules.categories.save")}</Button
        >
      </div>
    </form>
  {/if}
</Modal>

<Modal
  open={Boolean(removing)}
  title={t("rules.categories.delete_title")}
  onclose={() => (removing = null)}
>
  <div class="form">
    <p class="hint">
      {t("rules.categories.delete_message", { name: removing ? categoryLabel(removing) : "" })}
    </p>
    <div class="actions">
      <Button size="sm" onclick={() => (removing = null)}>{t("common.cancel")}</Button>
      <Button size="sm" variant="primary" loading={saving} onclick={remove}
        >{t("rules.categories.delete")}</Button
      >
    </div>
  </div>
</Modal>

<style>
  .categories,
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .hint {
    margin: 0;
    color: var(--text-muted);
    font-size: var(--text-xs);
    line-height: 1.5;
    overflow-wrap: break-word;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .category-row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-sm);
  }
  .category-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    color: var(--category-color, inherit);
  }
  .choice-group,
  .preview-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
  }
  .choices {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .choice {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    padding: 0;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    background: var(--surface-sunken);
    color: var(--text-muted);
    cursor: pointer;
  }
  .choice.selected {
    border-color: var(--ink);
    color: var(--ink);
  }
  .choice:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }
  .swatch {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    border-radius: 50%;
    background: var(--choice-color);
    color: var(--surface);
  }
  .preview-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }
  .preview {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background: color-mix(in srgb, var(--category-color) 18%, var(--surface));
    color: var(--ink);
  }
  .preview-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
  }
  .preview-rule {
    margin: 0 var(--space-2);
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    box-shadow: var(--elev-1);
    color: var(--ink);
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    overflow-wrap: anywhere;
  }
  .preview-name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }
</style>
