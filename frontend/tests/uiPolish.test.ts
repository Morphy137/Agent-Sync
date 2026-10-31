import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";
import { describeError } from "../src/lib/i18n/errors";
import { shouldAllowContextMenu } from "../src/lib/contextMenu";
import { aliasFromFolder, slugifyAlias } from "../src/lib/components/rules/ruleUi";
import { applyTheme, resolveTheme, FEATURED_THEME_IDS, THEMES } from "../src/lib/theme/theme";
import { selectionFromPaths, uniqueAliasForFolder } from "../src/lib/components/rules/ruleUi";

const mockApi = vi.hoisted(() => ({
  getGitignore: vi.fn(),
  saveGitignore: vi.fn(),
  addRecommendedGitignore: vi.fn(),
  excludeRepoPath: vi.fn(),
  selectFolder: vi.fn(),
  selectFile: vi.fn(),
}));

vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));

import Select from "../src/lib/components/Select.svelte";
import GitignoreModal from "../src/lib/components/GitignoreModal.svelte";

describe("describeError", () => {
  beforeEach(() => setLocale("en"));

  it("translates coded backend errors instead of showing the raw code", () => {
    const message = describeError('[root_not_found] {"path":"C:\\missing"}');
    expect(message).not.toContain("root_not_found");
    expect(message).toBe("The specified root folder was not found or is inaccessible.");
  });

  it("keeps plain messages", () => {
    expect(describeError("dialog exploded")).toBe("dialog exploded");
  });
});

describe("context menu guard", () => {
  it("blocks the browser menu on regular UI and allows it on fields or selected text", () => {
    const div = document.createElement("div");
    const input = document.createElement("input");
    expect(shouldAllowContextMenu(div, "")).toBe(false);
    expect(shouldAllowContextMenu(input, "")).toBe(true);
    expect(shouldAllowContextMenu(div, "C:/some/path")).toBe(true);
  });
});

describe("location alias helpers", () => {
  it("coerces typed text into a valid alias", () => {
    expect(slugifyAlias("AGENTS.md")).toBe("agents-md");
    expect(slugifyAlias("My Vault")).toBe("my-vault");
  });

  it("suggests an alias from the folder name", () => {
    expect(aliasFromFolder(String.raw`C:\Users\me\.claude`)).toBe("claude");
    expect(aliasFromFolder("/home/me/Obsidian Vault/")).toBe("obsidian-vault");
  });
});

describe("themes", () => {
  it("applies a full color set and its light/dark scheme", () => {
    applyTheme("nord");
    const root = document.documentElement;
    expect(root.getAttribute("data-theme")).toBe("nord");
    expect(root.getAttribute("data-scheme")).toBe("dark");
    expect(root.style.getPropertyValue("--canvas")).toBe("#242933");
    applyTheme("sakura");
    expect(root.getAttribute("data-scheme")).toBe("light");
  });

  it("resolves system and unknown ids to white or black", () => {
    expect(["light", "dark"]).toContain(resolveTheme("system").id);
    expect(["light", "dark"]).toContain(resolveTheme("nope").id);
  });

  it("features five themes and defines every color for all of them", () => {
    expect(FEATURED_THEME_IDS).toEqual(["light", "dark", "nord", "rose", "forest"]);
    for (const theme of THEMES) {
      for (const value of Object.values(theme.colors)) {
        expect(value).toMatch(/^#[0-9a-f]{6}$/i);
      }
    }
  });
});

describe("multi-file selection", () => {
  it("groups repository files under their common folder", () => {
    expect(selectionFromPaths(["skills/a/one.md", "skills/b/two.md"], false)).toEqual({
      base: "skills",
      include: ["a/one.md", "b/two.md"],
    });
    expect(selectionFromPaths(["README.md", "docs/x.md"], false)).toEqual({
      base: ".",
      include: ["README.md", "docs/x.md"],
    });
  });

  it("keeps Windows separators for local files and rejects different drives", () => {
    expect(
      selectionFromPaths([String.raw`C:\notes\a.txt`, String.raw`C:\notes\b.txt`], true)
    ).toEqual({ base: String.raw`C:\notes`, include: ["a.txt", "b.txt"] });
    expect(selectionFromPaths([String.raw`C:\a.txt`, String.raw`D:\b.txt`], true)).toBeNull();
  });
});

describe("location aliases from folders", () => {
  it("avoids clashes and the reserved home alias", () => {
    expect(uniqueAliasForFolder(String.raw`C:\Users\me\.claude`, ["claude"])).toBe("claude-2");
    expect(uniqueAliasForFolder("/home", [])).toBe("location");
  });
});

describe("Select.svelte", () => {
  afterEach(() => cleanup());

  const options = [
    { value: "a", label: "Alpha" },
    { value: "b", label: "Beta" },
    { value: "c", label: "Gamma", disabled: true },
  ];

  it("opens a styled listbox and selects with the mouse", async () => {
    const onchange = vi.fn();
    const { getByRole, findByRole } = render(Select, {
      value: "a",
      options,
      label: "Letter",
      onchange,
    });
    await fireEvent.click(getByRole("combobox", { name: /Letter/ }));
    await fireEvent.click(await findByRole("option", { name: "Beta" }));
    expect(onchange).toHaveBeenCalledWith("b");
  });

  it("supports keyboard navigation and skips disabled options", async () => {
    const onchange = vi.fn();
    const { getByRole, queryByRole } = render(Select, {
      value: "a",
      options,
      label: "Letter",
      onchange,
    });
    const trigger = getByRole("combobox", { name: /Letter/ });
    await fireEvent.keyDown(trigger, { key: "ArrowDown" });
    await fireEvent.keyDown(trigger, { key: "ArrowDown" });
    await fireEvent.keyDown(trigger, { key: "ArrowDown" });
    await fireEvent.keyDown(trigger, { key: "Enter" });
    expect(onchange).toHaveBeenCalledWith("b");
    expect(queryByRole("listbox")).toBeNull();
  });
});

describe("GitignoreModal.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    mockApi.getGitignore.mockResolvedValue({
      path: "C:/repo/.gitignore",
      exists: true,
      content: ".DS_Store\n",
      missingRecommended: [".env", "*.pem"],
    });
  });

  afterEach(() => cleanup());

  it("adds the recommended exclusions", async () => {
    mockApi.addRecommendedGitignore.mockResolvedValue({
      path: "C:/repo/.gitignore",
      exists: true,
      content: ".DS_Store\n.env\n*.pem\n",
      missingRecommended: [],
    });
    const { findByText, getByRole } = render(GitignoreModal, { open: true, onclose: vi.fn() });
    expect(await findByText("2 recommended exclusion(s) missing.")).toBeDefined();
    await fireEvent.click(getByRole("button", { name: "Add recommended" }));
    await waitFor(() => expect(mockApi.addRecommendedGitignore).toHaveBeenCalled());
    expect(await findByText("All recommended exclusions are present.")).toBeDefined();
  });

  it("excludes a picked folder", async () => {
    mockApi.selectFolder.mockResolvedValue("C:/repo/private");
    mockApi.excludeRepoPath.mockResolvedValue({
      path: "C:/repo/.gitignore",
      exists: true,
      content: ".DS_Store\n/private/\n",
      missingRecommended: [],
    });
    const { findByRole } = render(GitignoreModal, { open: true, onclose: vi.fn() });
    await fireEvent.click(await findByRole("button", { name: "Exclude a folder..." }));
    await waitFor(() => expect(mockApi.excludeRepoPath).toHaveBeenCalledWith("C:/repo/private"));
  });
});
