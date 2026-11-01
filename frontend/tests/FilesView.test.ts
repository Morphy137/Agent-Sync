import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, waitFor } from "@testing-library/svelte";
import FilesView from "../src/lib/views/FilesView.svelte";
import { api } from "../src/lib/api";
import { appState } from "../src/lib/stores/appState.svelte";
import { setLocale } from "../src/lib/i18n";

describe("files view", () => {
  beforeEach(() => {
    setLocale("en");
    appState.fileRuleId = "rule1";
    appState.fileDirty = false;
    vi.spyOn(api, "getRulesReport").mockResolvedValue({
      rules: [{ rule: { id: "rule1", name: "Instructions", source: { path: "AGENTS.md" } } }],
    } as any);
    vi.spyOn(api, "listRuleSourceFiles").mockResolvedValue([
      { path: ".", name: "AGENTS.md", absolutePath: "C:/repo/AGENTS.md" },
    ]);
  });
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    appState.fileRuleId = "";
    appState.fileDirty = false;
  });

  it("opens the selected rule and saves edited content using the original hash", async () => {
    vi.spyOn(api, "readRuleSourceFile").mockResolvedValue({
      content: "before",
      hash: "old-hash",
      readOnly: false,
      absolutePath: "C:/repo/AGENTS.md",
    });
    const save = vi.spyOn(api, "saveRuleSourceFile").mockResolvedValue("new-hash");
    const view = render(FilesView);
    const editor = await view.findByRole("textbox", { name: "Edit AGENTS.md" });
    await fireEvent.input(editor, { target: { value: "after" } });
    await fireEvent.click(view.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(save).toHaveBeenCalledWith("rule1", ".", "after", "old-hash"));
  });

  it("does not allow editing a subscription source", async () => {
    vi.spyOn(api, "readRuleSourceFile").mockResolvedValue({
      content: "shared",
      hash: "hash",
      readOnly: true,
      absolutePath: "C:/repo/AGENTS.md",
    });
    const view = render(FilesView);
    const editor = await view.findByRole("textbox", { name: "Edit AGENTS.md" });
    expect(editor.hasAttribute("readonly")).toBe(true);
    expect(view.queryByRole("button", { name: "Save" })).toBeNull();
  });

  it("preserves Windows line endings when saving", async () => {
    vi.spyOn(api, "readRuleSourceFile").mockResolvedValue({
      content: "first\r\nsecond\r\n",
      hash: "hash",
      readOnly: false,
      absolutePath: "C:/repo/AGENTS.md",
    });
    const save = vi.spyOn(api, "saveRuleSourceFile").mockResolvedValue("new-hash");
    const view = render(FilesView);
    const editor = await view.findByRole("textbox", { name: "Edit AGENTS.md" });
    await fireEvent.input(editor, { target: { value: "first\nchanged\n" } });
    await fireEvent.click(view.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith("rule1", ".", "first\r\nchanged\r\n", "hash")
    );
  });
});
