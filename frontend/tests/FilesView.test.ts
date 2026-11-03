import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, waitFor } from "@testing-library/svelte";
import FilesView from "../src/lib/views/FilesView.svelte";
import { api } from "../src/lib/api";
import { appState } from "../src/lib/stores/appState.svelte";
import { setLocale } from "../src/lib/i18n";

/** Markdown opens in the reader; the eye switches the file to the editor. */
async function openEditor(view: ReturnType<typeof render>, name: string) {
  await fireEvent.click(await view.findByRole("button", { name: "Markdown preview" }));
  return (await view.findByRole("textbox", { name: `Edit ${name}` })) as HTMLTextAreaElement;
}

describe("files view", () => {
  beforeEach(() => {
    setLocale("en");
    appState.fileRuleId = "rule1";
    appState.fileDirty = false;
    vi.spyOn(api, "listFileRoots").mockResolvedValue([
      { id: "repo", name: "My repository" },
    ] as any);
    vi.spyOn(api, "listFileDirectory").mockResolvedValue([
      { path: "AGENTS.md", name: "AGENTS.md", isDir: false },
    ] as any);
    vi.spyOn(api, "getRulesReport").mockResolvedValue({
      rules: [
        {
          rule: {
            id: "rule1",
            name: "Instructions",
            source: { origin: "repo", path: "AGENTS.md" },
          },
        },
      ],
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
    vi.spyOn(api, "readBrowserFile").mockResolvedValue({
      content: "before",
      hash: "old-hash",
      readOnly: false,
      absolutePath: "C:/repo/AGENTS.md",
    });
    const save = vi.spyOn(api, "saveBrowserFile").mockResolvedValue("new-hash");
    const view = render(FilesView);
    const editor = await openEditor(view, "AGENTS.md");
    await fireEvent.input(editor, { target: { value: "after" } });
    await fireEvent.click(view.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(
        "repo",
        "AGENTS.md",
        "after",
        "old-hash",
        "C:/repo/AGENTS.md"
      )
    );
  });

  it("respects a read-only source", async () => {
    vi.spyOn(api, "readBrowserFile").mockResolvedValue({
      content: "shared",
      hash: "hash",
      readOnly: true,
      absolutePath: "C:/repo/AGENTS.md",
    });
    const view = render(FilesView);
    expect(await view.findByText("shared")).toBeDefined();
    const editor = await openEditor(view, "AGENTS.md");
    expect(editor.hasAttribute("readonly")).toBe(true);
    expect(view.queryByRole("button", { name: "Save" })).toBeNull();
    await fireEvent.click(view.getByRole("button", { name: "Markdown preview" }));
    expect(view.getByText("shared")).toBeDefined();
  });

  it("preserves Windows line endings when saving", async () => {
    vi.spyOn(api, "readBrowserFile").mockResolvedValue({
      content: "first\r\nsecond\r\n",
      hash: "hash",
      readOnly: false,
      absolutePath: "C:/repo/AGENTS.md",
    });
    const save = vi.spyOn(api, "saveBrowserFile").mockResolvedValue("new-hash");
    const view = render(FilesView);
    const editor = await openEditor(view, "AGENTS.md");
    await fireEvent.input(editor, { target: { value: "first\nchanged\n" } });
    await fireEvent.click(view.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(
        "repo",
        "AGENTS.md",
        "first\r\nchanged\r\n",
        "hash",
        "C:/repo/AGENTS.md"
      )
    );
  });

  it("previews unsaved Markdown safely and returns to the editor", async () => {
    vi.spyOn(api, "readBrowserFile").mockResolvedValue({
      content: "# Original",
      hash: "hash",
      readOnly: false,
      absolutePath: "C:/repo/AGENTS.md",
    });
    const view = render(FilesView);
    expect(await view.findByRole("heading", { name: "Original" })).toBeDefined();
    const editor = await openEditor(view, "AGENTS.md");
    await fireEvent.input(editor, {
      target: { value: '# Changed\n\n**Bold**\n\n<img src="x" onerror="alert(1)">' },
    });
    const toggle = view.getByRole("button", { name: "Markdown preview" });
    await fireEvent.click(toggle);
    expect(view.getByRole("heading", { name: "Changed" })).toBeDefined();
    expect(view.getByText("Bold").tagName).toBe("STRONG");
    expect(view.container.querySelector("img")?.hasAttribute("onerror")).toBe(false);
    expect(toggle.getAttribute("aria-pressed")).toBe("true");
    await fireEvent.click(toggle);
    expect(
      (view.getByRole("textbox", { name: "Edit AGENTS.md" }) as HTMLTextAreaElement).value
    ).toContain("# Changed");
    expect(view.getByRole("button", { name: "Save" }).hasAttribute("disabled")).toBe(false);
  });

  it("navigates repositories and folders and opens files outside rules", async () => {
    appState.fileRuleId = "";
    const list = vi.mocked(api.listFileDirectory);
    list.mockImplementation(async (_root, path) =>
      path === "."
        ? [{ path: "docs", name: "docs", isDir: true }]
        : [{ path: "docs/notes.txt", name: "notes.txt", isDir: false }]
    );
    vi.spyOn(api, "readBrowserFile").mockResolvedValue({
      content: "Hello",
      hash: "hash",
      readOnly: false,
      absolutePath: "C:/repo/docs/notes.txt",
    });
    const view = render(FilesView);
    await fireEvent.click(await view.findByRole("button", { name: "My repository" }));
    await fireEvent.click(await view.findByRole("button", { name: "docs" }));
    await fireEvent.click(await view.findByRole("button", { name: "notes.txt" }));
    const editor = (await view.findByRole("textbox", {
      name: "Edit notes.txt",
    })) as HTMLTextAreaElement;
    await fireEvent.input(editor, { target: { value: "Unsaved" } });
    expect(view.queryByRole("button", { name: "Markdown preview" })).toBeNull();
    await fireEvent.click(view.getByRole("button", { name: "Back" }));
    expect(await view.findByRole("button", { name: "docs" })).toBeDefined();
    await fireEvent.click(view.getByRole("button", { name: "All repositories" }));
    expect(await view.findByRole("button", { name: "My repository" })).toBeDefined();
    expect(editor.value).toBe("Unsaved");
    await fireEvent.click(view.getByRole("button", { name: "Refresh files" }));
    expect(api.listFileRoots).toHaveBeenCalledTimes(3);
  });

  it("keeps navigation controls enabled while a file is loading", async () => {
    appState.fileRuleId = "";
    let resolveFile!: (value: any) => void;
    vi.spyOn(api, "readBrowserFile").mockImplementation(
      () => new Promise((resolve) => (resolveFile = resolve))
    );
    const view = render(FilesView);
    await fireEvent.click(await view.findByRole("button", { name: "My repository" }));
    await fireEvent.click(await view.findByRole("button", { name: "AGENTS.md" }));
    for (const name of ["Back", "All repositories", "Refresh files"]) {
      expect(view.getByRole("button", { name }).hasAttribute("disabled")).toBe(false);
    }
    resolveFile({ content: "text", hash: "h", absolutePath: "C:/repo/AGENTS.md", readOnly: false });
    await view.findByRole("document");
  });

  it("offers JSON formatting while preserving unsaved edits", async () => {
    appState.fileRuleId = "";
    vi.mocked(api.listFileDirectory).mockResolvedValue([
      { path: "settings.json", name: "settings.json", isDir: false },
    ]);
    vi.spyOn(api, "readBrowserFile").mockResolvedValue({
      content: '{"enabled":true}',
      hash: "hash",
      readOnly: false,
      absolutePath: "C:/repo/settings.json",
    });
    const view = render(FilesView);
    await fireEvent.click(await view.findByRole("button", { name: "My repository" }));
    await fireEvent.click(await view.findByRole("button", { name: "settings.json" }));
    const editor = (await view.findByRole("textbox", {
      name: "Edit settings.json",
    })) as HTMLTextAreaElement;
    await fireEvent.click(view.getByRole("button", { name: "Format JSON" }));
    expect(editor.value).toBe('{\n  "enabled": true\n}');
    expect(view.getByRole("button", { name: "Save" }).hasAttribute("disabled")).toBe(false);
  });

  describe("reader mode", () => {
    const entries = [
      { path: "AGENTS.md", name: "AGENTS.md", isDir: false },
      { path: "NOTES.md", name: "NOTES.md", isDir: false },
      { path: "notes.txt", name: "notes.txt", isDir: false },
    ];

    beforeEach(() => {
      appState.fileRuleId = "";
      vi.mocked(api.listFileDirectory).mockResolvedValue(entries as any);
      vi.spyOn(api, "readBrowserFile").mockImplementation(async (_root, path) => ({
        content: path.endsWith(".md") ? `# ${path}` : `plain ${path}`,
        hash: "hash",
        readOnly: false,
        absolutePath: `C:/repo/${path}`,
      }));
    });

    async function openRepository() {
      const view = render(FilesView);
      await fireEvent.click(await view.findByRole("button", { name: "My repository" }));
      return view;
    }

    it("opens every Markdown file as a preview, again each time another file is opened", async () => {
      const view = await openRepository();
      await fireEvent.click(await view.findByRole("button", { name: "AGENTS.md" }));
      expect(await view.findByRole("heading", { name: "AGENTS.md" })).toBeDefined();
      expect(view.queryByRole("textbox")).toBeNull();
      const eye = view.getByRole("button", { name: "Markdown preview" });
      expect(eye.getAttribute("aria-pressed")).toBe("true");

      await openEditor(view, "AGENTS.md");
      await fireEvent.click(view.getByRole("button", { name: "NOTES.md" }));
      expect(await view.findByRole("heading", { name: "NOTES.md" })).toBeDefined();
      expect(view.queryByRole("textbox")).toBeNull();
    });

    it("lets the eye switch a single file to the editor and back", async () => {
      const view = await openRepository();
      await fireEvent.click(await view.findByRole("button", { name: "AGENTS.md" }));
      const editor = await openEditor(view, "AGENTS.md");
      expect(editor.value).toBe("# AGENTS.md");
      const eye = view.getByRole("button", { name: "Markdown preview" });
      expect(eye.getAttribute("aria-pressed")).toBe("false");
      await fireEvent.click(eye);
      expect(view.queryByRole("textbox")).toBeNull();
      expect(view.getByRole("document")).toBeDefined();
    });

    it("opens files without a preview straight in the editor", async () => {
      const view = await openRepository();
      await fireEvent.click(await view.findByRole("button", { name: "notes.txt" }));
      const editor = (await view.findByRole("textbox", {
        name: "Edit notes.txt",
      })) as HTMLTextAreaElement;
      expect(editor.value).toBe("plain notes.txt");
      expect(view.queryByRole("button", { name: "Markdown preview" })).toBeNull();
    });

    it("keeps a dirty file in the editor when leaving it is declined", async () => {
      const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
      const view = await openRepository();
      await fireEvent.click(await view.findByRole("button", { name: "AGENTS.md" }));
      const editor = await openEditor(view, "AGENTS.md");
      await fireEvent.input(editor, { target: { value: "# Unsaved" } });

      await fireEvent.click(view.getByRole("button", { name: "NOTES.md" }));
      expect(confirm).toHaveBeenCalled();
      const still = view.getByRole("textbox", { name: "Edit AGENTS.md" }) as HTMLTextAreaElement;
      expect(still.value).toBe("# Unsaved");
      expect(view.queryByRole("document")).toBeNull();
    });
  });
});
