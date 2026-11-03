import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, waitFor } from "@testing-library/svelte";
import RuleSourceViewer from "../src/lib/components/rules/RuleSourceViewer.svelte";
import RulesView from "../src/lib/views/RulesView.svelte";
import { api } from "../src/lib/api";
import { appState } from "../src/lib/stores/appState.svelte";
import { setLocale } from "../src/lib/i18n";

const skillFiles = [
  { path: "a/notes.txt", name: "notes.txt", absolutePath: "C:/s/a/notes.txt" },
  { path: "README.md", name: "README.md", absolutePath: "C:/s/README.md" },
  { path: "SKILL.md", name: "SKILL.md", absolutePath: "C:/s/SKILL.md" },
];

function content(text: string) {
  return { content: text, hash: "h", readOnly: true, absolutePath: "C:/s/x" };
}

function renderViewer(props: Partial<{ initialPath: string; ruleId: string }> = {}) {
  const onclose = vi.fn();
  const view = render(RuleSourceViewer, {
    open: true,
    ruleId: "r1",
    ruleName: "Skills",
    onclose,
    ...props,
  });
  return { ...view, onclose };
}

describe("RuleSourceViewer", () => {
  beforeEach(() => setLocale("en"));
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("shows a single-file rule without a sidebar and renders Markdown", async () => {
    vi.spyOn(api, "listRuleSourceFiles").mockResolvedValue([
      { path: ".", name: "AGENTS.md", absolutePath: "C:/s/AGENTS.md" },
    ]);
    const read = vi.spyOn(api, "readRuleSourceFile").mockResolvedValue(content("# Hello"));
    const view = renderViewer();
    expect(await view.findByRole("heading", { name: "Hello" })).toBeDefined();
    expect(read).toHaveBeenCalledWith("r1", ".");
    expect(view.queryByRole("navigation")).toBeNull();
    expect(view.getByText("AGENTS.md")).toBeDefined();
  });

  it("is strictly read-only: no editor and no save control", async () => {
    vi.spyOn(api, "listRuleSourceFiles").mockResolvedValue(skillFiles);
    vi.spyOn(api, "readRuleSourceFile").mockResolvedValue(content("text"));
    const view = renderViewer();
    await view.findByRole("document");
    expect(view.queryByRole("textbox")).toBeNull();
    expect(view.queryByRole("button", { name: /save/i })).toBeNull();
    expect(view.container.querySelector("textarea")).toBeNull();
    expect(view.getByText("Read only")).toBeDefined();
  });

  it("lists the files of a multi-file rule and opens SKILL.md first", async () => {
    vi.spyOn(api, "listRuleSourceFiles").mockResolvedValue(skillFiles);
    const read = vi
      .spyOn(api, "readRuleSourceFile")
      .mockImplementation(async (_id, path) => content(path === "SKILL.md" ? "# Skill" : "plain"));
    const view = renderViewer();
    expect(await view.findByRole("heading", { name: "Skill" })).toBeDefined();
    expect(read).toHaveBeenCalledTimes(1);
    const nav = view.getByRole("navigation", { name: "Source files" });
    const items = [...nav.querySelectorAll("button")];
    expect(items.map((item) => item.textContent?.trim())).toEqual([
      "a/notes.txt",
      "README.md",
      "SKILL.md",
    ]);
    expect(items[2].getAttribute("aria-current")).toBe("true");

    await fireEvent.click(items[0]);
    await waitFor(() => expect(items[0].getAttribute("aria-current")).toBe("true"));
    expect(items[2].hasAttribute("aria-current")).toBe(false);
    const plain = await view.findByText("plain");
    expect(plain.tagName).toBe("PRE");
    expect(view.queryByRole("document")).toBeNull();
  });

  it("starts on the requested file", async () => {
    vi.spyOn(api, "listRuleSourceFiles").mockResolvedValue(skillFiles);
    const read = vi.spyOn(api, "readRuleSourceFile").mockResolvedValue(content("text"));
    const view = renderViewer({ initialPath: "README.md" });
    await view.findByText("text");
    expect(read).toHaveBeenCalledWith("r1", "README.md");
  });

  it("ignores the response of a file that was superseded", async () => {
    vi.spyOn(api, "listRuleSourceFiles").mockResolvedValue([
      { path: "a.txt", name: "a.txt", absolutePath: "C:/s/a.txt" },
      { path: "b.txt", name: "b.txt", absolutePath: "C:/s/b.txt" },
    ]);
    const pending: Record<string, (value: unknown) => void> = {};
    vi.spyOn(api, "readRuleSourceFile").mockImplementation(
      (_id, path) => new Promise((resolve) => (pending[path] = resolve as never))
    );
    const view = renderViewer();
    await waitFor(() => expect(pending["a.txt"]).toBeDefined());
    await fireEvent.click(view.getByRole("button", { name: "b.txt" }));
    await waitFor(() => expect(pending["b.txt"]).toBeDefined());
    pending["b.txt"](content("second"));
    expect(await view.findByText("second")).toBeDefined();
    pending["a.txt"](content("first"));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(view.queryByText("first")).toBeNull();
    expect(view.getByText("second")).toBeDefined();
  });

  it("explains files that cannot be shown", async () => {
    vi.spyOn(api, "listRuleSourceFiles").mockResolvedValue([
      { path: "big.txt", name: "big.txt", absolutePath: "C:/s/big.txt" },
      { path: "bin.dat", name: "bin.dat", absolutePath: "C:/s/bin.dat" },
      { path: "gone.txt", name: "gone.txt", absolutePath: "C:/s/gone.txt" },
    ]);
    vi.spyOn(api, "readRuleSourceFile").mockImplementation(async (_id, path) => {
      if (path === "big.txt") throw '[invalid_argument] {"reason":"file_too_large"}';
      if (path === "bin.dat") throw '[invalid_argument] {"reason":"file_not_text"}';
      throw "[source_not_found]";
    });
    const view = renderViewer();
    expect(await view.findByText("This file is too large to display.")).toBeDefined();
    await fireEvent.click(view.getByRole("button", { name: "bin.dat" }));
    expect(
      await view.findByText("This file is not text, so it cannot be displayed.")
    ).toBeDefined();
    await fireEvent.click(view.getByRole("button", { name: "gone.txt" }));
    expect(await view.findByText("This file could not be found in the source.")).toBeDefined();
  });

  it("shows an empty state for a folder without files", async () => {
    vi.spyOn(api, "listRuleSourceFiles").mockResolvedValue([]);
    const read = vi.spyOn(api, "readRuleSourceFile");
    const view = renderViewer();
    expect(await view.findByText("This source has no files to show.")).toBeDefined();
    expect(read).not.toHaveBeenCalled();
  });

  it("says so when the source cannot be listed", async () => {
    vi.spyOn(api, "listRuleSourceFiles").mockRejectedValue("[source_not_found]");
    const view = renderViewer();
    expect(await view.findByText("The source of this rule could not be found.")).toBeDefined();
  });
});

describe("rules list viewer", () => {
  const folderRule = (enabled: boolean) => ({
    id: "r1",
    name: "Skills",
    enabled,
    source: { origin: "subscription", subscription: "s1", path: "skills" },
    kind: "folder",
    destinations: [{ id: "d1", location: "claude", path: "skills", mode: "one_way", enabled }],
  });
  const report = (enabled: boolean) => ({
    rules: [
      {
        rule: folderRule(enabled),
        scope: "repo",
        detectedKind: "folder",
        destinations: [
          {
            ruleId: "r1",
            destId: "d1",
            state: enabled ? "ready" : "disabled",
            targetRoot: "C:/home/.claude",
            files: enabled
              ? [
                  { relPath: "a/SKILL.md", sourceAbs: "", targetAbs: "", status: "in_sync" },
                  { relPath: "b/SKILL.md", sourceAbs: "", targetAbs: "", status: "in_sync" },
                ]
              : [],
          },
        ],
      },
    ],
    collisions: [],
    unmapped: [],
    pending: 0,
    conflicts: 0,
  });

  beforeEach(() => {
    setLocale("en");
    appState.config = { rootPath: "C:/repo" } as never;
    vi.spyOn(api, "getStatus").mockResolvedValue({ rulesPending: 0 } as never);
    vi.spyOn(api, "listRuleSourceFiles").mockResolvedValue([
      { path: "a/SKILL.md", name: "SKILL.md", absolutePath: "C:/s/a/SKILL.md" },
      { path: "b/SKILL.md", name: "SKILL.md", absolutePath: "C:/s/b/SKILL.md" },
    ]);
    vi.spyOn(api, "readRuleSourceFile").mockImplementation(async (_id, path) =>
      content(`# ${path}`)
    );
  });
  afterEach(() => {
    cleanup();
    appState.ruleReport = null;
    vi.restoreAllMocks();
  });

  it("opens from a file in the expanded list with that file selected", async () => {
    vi.spyOn(api, "getRulesReport").mockResolvedValue(report(true) as never);
    const view = render(RulesView);
    await fireEvent.click(await view.findByRole("button", { name: "Show details for Skills" }));
    await fireEvent.click(view.getByRole("button", { name: "View b/SKILL.md" }));
    expect(await view.findByRole("heading", { name: "b/SKILL.md" })).toBeDefined();
    const nav = view.getByRole("navigation", { name: "Source files" });
    expect(nav.querySelector("[aria-current]")?.textContent?.trim()).toBe("b/SKILL.md");
  });

  it("opens for a disabled rule by listing the source itself", async () => {
    vi.spyOn(api, "getRulesReport").mockResolvedValue(report(false) as never);
    const view = render(RulesView);
    await fireEvent.click(await view.findByRole("button", { name: "View source of Skills" }));
    expect(await view.findByRole("heading", { name: "a/SKILL.md" })).toBeDefined();
    expect(api.listRuleSourceFiles).toHaveBeenCalledWith("r1");
    expect(view.getByRole("navigation", { name: "Source files" })).toBeDefined();
  });
});
