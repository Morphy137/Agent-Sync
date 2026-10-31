import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";

const mockApi = vi.hoisted(() => ({
  listRepoTree: vi.fn(),
  listSubscriptions: vi.fn(),
  getSubscriptionTree: vi.fn(),
  selectFiles: vi.fn(),
  selectFolder: vi.fn(),
  saveLocation: vi.fn(),
  importIntoRepo: vi.fn(),
}));

vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));

import NotificationCenter from "../src/lib/components/notifications/NotificationCenter.svelte";
import SourcePicker from "../src/lib/components/rules/SourcePicker.svelte";
import LocationSelect from "../src/lib/components/rules/LocationSelect.svelte";

describe("NotificationCenter.svelte", () => {
  beforeEach(() => setLocale("en"));
  afterEach(() => cleanup());

  const items = [
    {
      id: "no_remote",
      variant: "info" as const,
      title: "No Remote",
      message: "Sync between PCs is off.",
      actions: [{ label: "Connect", primary: true, onclick: vi.fn() }],
    },
  ];

  it("shows a closable popup and keeps the item under the bell", async () => {
    const { getByRole, queryByRole, getByText } = render(NotificationCenter, { items });

    expect(getByRole("status")).toBeDefined();
    await fireEvent.click(getByRole("button", { name: "Close notification" }));
    expect(queryByRole("status")).toBeNull();

    await fireEvent.click(getByRole("button", { name: "Notifications (1)" }));
    expect(getByRole("dialog", { name: "Notifications" })).toBeDefined();
    expect(getByText("Sync between PCs is off.")).toBeDefined();
  });

  it("runs an action from the popup", async () => {
    const { getByRole } = render(NotificationCenter, { items });
    await fireEvent.click(getByRole("button", { name: "Connect" }));
    expect(items[0].actions[0].onclick).toHaveBeenCalled();
  });
});

describe("SourcePicker.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
  });
  afterEach(() => cleanup());

  it("turns several local files into a folder plus a selection", async () => {
    mockApi.selectFiles.mockResolvedValue(["C:\\notes\\a.txt", "C:\\notes\\b.md"]);
    const onchange = vi.fn();
    const { getByRole } = render(SourcePicker, { value: null, hasRoot: false, onchange });

    await fireEvent.click(getByRole("button", { name: "Choose files" }));
    await waitFor(() => expect(onchange).toHaveBeenCalled());
    expect(onchange.mock.calls[0][0]).toMatchObject({
      origin: "local",
      path: "C:\\notes",
      include: ["a.txt", "b.md"],
    });
  });

  it("shows selected files compactly and removes one", async () => {
    const onchange = vi.fn();
    const { getByText, getByRole } = render(SourcePicker, {
      value: { origin: "local", path: "C:\\notes", include: ["a.txt", "b.md", "c.md"] } as any,
      hasRoot: false,
      onchange,
    });

    expect(getByText("a.txt")).toBeDefined();
    expect(getByText("3 item(s)")).toBeDefined();
    await fireEvent.click(getByRole("button", { name: "Remove a.txt" }));
    expect(onchange.mock.calls[0][0]).toMatchObject({
      path: "C:\\notes",
      include: ["b.md", "c.md"],
    });
  });

  it("selects several repository files with checkboxes", async () => {
    mockApi.listRepoTree.mockResolvedValue([
      { path: "README.md", isDir: false, size: 1, kind: "file" },
      { path: "AGENTS.md", isDir: false, size: 1, kind: "instructions" },
    ]);
    let value: any = null;
    const onchange = vi.fn((source) => (value = source));
    const view = render(SourcePicker, { value, hasRoot: true, onchange });

    await fireEvent.click(view.getByRole("switch", { name: "Select several files" }));
    await fireEvent.click(await view.findByRole("checkbox", { name: /README\.md/ }));
    await view.rerender({ value, hasRoot: true, onchange });
    await fireEvent.click(view.getByRole("checkbox", { name: /AGENTS\.md/ }));

    expect(onchange.mock.calls.at(-1)![0]).toMatchObject({
      origin: "repo",
      path: ".",
      include: ["AGENTS.md", "README.md"],
    });
  });
});

describe("LocationSelect.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
  });
  afterEach(() => cleanup());

  const locations = [
    { alias: "home", path: "C:/home", mapped: true, builtin: true },
    { alias: "claude", path: "C:/home/.claude", mapped: true, builtin: false },
  ];

  it("creates a location straight from the folder dialog", async () => {
    mockApi.selectFolder.mockResolvedValue("D:\\Vaults\\Obsidian Notes");
    mockApi.saveLocation.mockResolvedValue({});
    const oncreated = vi.fn();
    const { getByRole, findByRole } = render(LocationSelect, {
      value: "",
      locations,
      onselect: vi.fn(),
      oncreated,
    });

    await fireEvent.click(getByRole("combobox", { name: /Location/ }));
    await fireEvent.click(await findByRole("option", { name: "New location..." }));
    await waitFor(() =>
      expect(mockApi.saveLocation).toHaveBeenCalledWith(
        "obsidian-notes",
        "D:\\Vaults\\Obsidian Notes"
      )
    );
    expect(oncreated).toHaveBeenCalledWith("obsidian-notes");
  });

  it("reuses an existing location for the same folder", async () => {
    mockApi.selectFolder.mockResolvedValue("C:\\home\\.claude\\");
    const onselect = vi.fn();
    const { getByRole, findByRole } = render(LocationSelect, {
      value: "",
      locations,
      onselect,
      oncreated: vi.fn(),
    });

    await fireEvent.click(getByRole("combobox", { name: /Location/ }));
    await fireEvent.click(await findByRole("option", { name: "New location..." }));
    await waitFor(() => expect(onselect).toHaveBeenCalledWith("claude"));
    expect(mockApi.saveLocation).not.toHaveBeenCalled();
  });
});
