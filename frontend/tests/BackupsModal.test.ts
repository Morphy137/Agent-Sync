import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import BackupsModal from "../src/lib/components/backups/BackupsModal.svelte";
import {
  SEARCH_THRESHOLD,
  filterBackups,
  formatSize,
  groupBackups,
} from "../src/lib/components/backups/backups";
import { appState } from "../src/lib/stores/appState.svelte";
import { setLocale } from "../src/lib/i18n";

const mockApi = vi.hoisted(() => ({
  listBackups: vi.fn(),
  restoreBackup: vi.fn(),
  openPath: vi.fn(),
}));
vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));

function entry(overrides: Record<string, unknown> = {}) {
  return {
    id: "rules/r1__d1/claude/110326-042530PM",
    kind: "rule",
    owner: "Agent instructions",
    destination: "claude/CLAUDE.md",
    ruleId: "r1",
    destId: "d1",
    targetPath: "C:/Users/me/.claude",
    createdAt: "2026-11-03T16:25:30Z",
    reason: "sync",
    fileCount: 2,
    totalSize: 2048,
    files: ["CLAUDE.md", "docs/extra.md"],
    path: "C:/backups/rules/r1__d1/claude/110326-042530PM",
    restorable: true,
    ...overrides,
  };
}

function renderModal() {
  return render(BackupsModal, { props: { open: true, onclose: vi.fn() } });
}

describe("BackupsModal.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    appState.toasts = [];
    appState.alertNotifications = [];
    mockApi.listBackups.mockResolvedValue([entry()]);
    mockApi.restoreBackup.mockResolvedValue({
      restored: ["CLAUDE.md", "docs/extra.md"],
      target: "C:/Users/me/.claude",
      undoBackupId: "rules/r1__d1/claude/110326-050000PM",
    });
    vi.spyOn(appState, "refreshLocalStatus").mockResolvedValue();
    vi.spyOn(appState, "bumpRules").mockImplementation(() => {});
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("shows an empty state when there are no backups", async () => {
    mockApi.listBackups.mockResolvedValue([]);
    const { findByText } = renderModal();
    expect(await findByText("No backups yet")).toBeTruthy();
  });

  it("lists backups with owner, reason, file count and size, then shows files on select", async () => {
    const { findByText, getByText, queryByText } = renderModal();
    expect(await findByText("Agent instructions")).toBeTruthy();
    expect(getByText(/Before a sync · 2 files · 2.0 KB/)).toBeTruthy();
    expect(queryByText("docs/extra.md")).toBeNull();

    await fireEvent.click(getByText(/Before a sync/));
    expect(getByText("docs/extra.md")).toBeTruthy();
    expect(getByText("Restore all")).toBeTruthy();
  });

  it("restores everything after confirming, then refreshes status and rules", async () => {
    const { findByText, getByText } = renderModal();
    await fireEvent.click(await findByText(/Before a sync/));

    await fireEvent.click(getByText("Restore all"));
    expect(getByText(/Restore 2 files to C:\/Users\/me\/\.claude/)).toBeTruthy();
    expect(getByText(/backed up first/)).toBeTruthy();
    expect(mockApi.restoreBackup).not.toHaveBeenCalled();

    await fireEvent.click(getByText("Restore"));
    await waitFor(() => expect(mockApi.restoreBackup).toHaveBeenCalledTimes(1));
    expect(mockApi.restoreBackup).toHaveBeenCalledWith(entry().id, []);
    await waitFor(() => expect(appState.bumpRules).toHaveBeenCalled());
    expect(appState.refreshLocalStatus).toHaveBeenCalled();
    expect(appState.toasts.some((toast) => toast.type === "success")).toBe(true);
    expect(mockApi.listBackups).toHaveBeenCalledTimes(2);
  });

  it("restores only the selected files", async () => {
    const { findByText, getByText, getByLabelText } = renderModal();
    await fireEvent.click(await findByText(/Before a sync/));

    await fireEvent.click(getByLabelText("docs/extra.md"));
    await fireEvent.click(getByText("Restore selected (1)"));
    await fireEvent.click(getByText("Restore"));
    await waitFor(() => expect(mockApi.restoreBackup).toHaveBeenCalled());
    expect(mockApi.restoreBackup).toHaveBeenCalledWith(entry().id, ["CLAUDE.md"]);
  });

  it("cancelling the confirmation restores nothing", async () => {
    const { findByText, getByText } = renderModal();
    await fireEvent.click(await findByText(/Before a sync/));
    await fireEvent.click(getByText("Restore all"));
    await fireEvent.click(getByText("Cancel"));
    expect(mockApi.restoreBackup).not.toHaveBeenCalled();
    expect(getByText("Restore all")).toBeTruthy();
  });

  it("reports a failed restore as a translated error and keeps the list", async () => {
    mockApi.restoreBackup.mockRejectedValue("[backup_not_restorable]");
    const { findByText, getByText } = renderModal();
    await fireEvent.click(await findByText(/Before a sync/));
    await fireEvent.click(getByText("Restore all"));
    await fireEvent.click(getByText("Restore"));
    await waitFor(() => expect(appState.alertNotifications.length).toBeGreaterThan(0));
    expect(appState.alertNotifications.at(-1)?.message).toMatch(/cannot be restored/);
    expect(appState.bumpRules).not.toHaveBeenCalled();
  });

  it("disables restore for backups that cannot be restored but still opens the folder", async () => {
    mockApi.listBackups.mockResolvedValue([entry({ restorable: false })]);
    const { findByText, getByText } = renderModal();
    await fireEvent.click(await findByText(/Before a sync/));
    expect(getByText(/cannot be restored from here/)).toBeTruthy();
    expect((getByText("Restore all").closest("button") as HTMLButtonElement).disabled).toBe(true);

    await fireEvent.click(getByText("Open folder"));
    expect(mockApi.openPath).toHaveBeenCalledWith(entry().path);
  });

  it("shows the search box only for long lists and filters by rule", async () => {
    const many = Array.from({ length: SEARCH_THRESHOLD + 1 }, (_, i) =>
      entry({
        id: `rules/r${i}__d1/claude/t${i}`,
        ruleId: `r${i}`,
        owner: i === 0 ? "Special rule" : `Rule ${i}`,
        createdAt: `2026-11-03T16:25:${String(10 + i).padStart(2, "0")}Z`,
      })
    );
    mockApi.listBackups.mockResolvedValue(many);
    const { findByPlaceholderText, getByText, queryByText } = renderModal();
    const search = await findByPlaceholderText("Search by rule or file");
    await fireEvent.input(search, { target: { value: "special" } });
    expect(getByText("Special rule")).toBeTruthy();
    expect(queryByText("Rule 3")).toBeNull();
    await fireEvent.input(search, { target: { value: "zzz" } });
    expect(getByText("No backups match your search.")).toBeTruthy();
  });

  it("does not show the search box for short lists", async () => {
    const { findByText, queryByPlaceholderText } = renderModal();
    await findByText("Agent instructions");
    expect(queryByPlaceholderText("Search by rule or file")).toBeNull();
  });
});

describe("backup list helpers", () => {
  it("formats sizes", () => {
    expect(formatSize(0)).toBe("0 B");
    expect(formatSize(1023)).toBe("1023 B");
    expect(formatSize(1536)).toBe("1.5 KB");
    expect(formatSize(5 * 1024 * 1024)).toBe("5.0 MB");
  });

  it("groups by rule and destination keeping newest-first order", () => {
    const items = [
      entry({ id: "a", ruleId: "r1", destId: "d1" }),
      entry({ id: "b", ruleId: "r2", destId: "d1", owner: "Other" }),
      entry({ id: "c", ruleId: "r1", destId: "d1" }),
    ] as any[];
    const groups = groupBackups(items);
    expect(groups.map((g) => g.entries.map((e) => e.id))).toEqual([["a", "c"], ["b"]]);
  });

  it("filters by file name too", () => {
    const items = [entry()] as any[];
    expect(filterBackups(items, "extra")).toHaveLength(1);
    expect(filterBackups(items, "nothing")).toHaveLength(0);
    expect(filterBackups(items, "  ")).toHaveLength(1);
  });
});
