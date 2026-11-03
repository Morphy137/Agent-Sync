import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";
import SyncPreviewModal from "../src/lib/components/SyncPreviewModal.svelte";

function planWithConflicts(paths: string[]) {
  return {
    fingerprint: "fp",
    direction: "conflict",
    conflicts: paths.map((path) => ({ path, reason: "remote_and_local_overlap" })),
    commitPreview: paths,
    ahead: 0,
    behind: 1,
  } as any;
}

describe("SyncPreviewModal.svelte", () => {
  beforeEach(() => setLocale("en"));
  afterEach(() => cleanup());

  it("lists each conflicting repository file and keeps the repository version by default", async () => {
    const onconfirm = vi.fn();
    const plan = planWithConflicts(["shared/AGENTS.md", "skills/review/SKILL.md"]);
    const { getByText, getByRole } = render(SyncPreviewModal, {
      open: true,
      plan,
      onclose: vi.fn(),
      onconfirm,
    });

    expect(getByText("shared/AGENTS.md")).toBeDefined();
    expect(getByText("skills/review/SKILL.md")).toBeDefined();

    await fireEvent.click(getByRole("button", { name: "Start Sync" }));
    expect(onconfirm).toHaveBeenCalledWith([
      { path: "shared/AGENTS.md", resolution: "keep_repo" },
      { path: "skills/review/SKILL.md", resolution: "keep_repo" },
    ]);
  });

  it("sends the choice made for every file", async () => {
    const onconfirm = vi.fn();
    const { getByRole } = render(SyncPreviewModal, {
      open: true,
      plan: planWithConflicts(["shared/AGENTS.md", "notes/todo.md"]),
      onclose: vi.fn(),
      onconfirm,
    });

    await fireEvent.click(getByRole("button", { name: "Apply This PC to all" }));
    await fireEvent.click(getByRole("button", { name: "Start Sync" }));
    expect(onconfirm).toHaveBeenCalledWith([
      { path: "shared/AGENTS.md", resolution: "keep_local" },
      { path: "notes/todo.md", resolution: "keep_local" },
    ]);
  });

  it("confirms without resolutions when nothing conflicts", async () => {
    const onconfirm = vi.fn();
    const { getByRole, queryByText } = render(SyncPreviewModal, {
      open: true,
      plan: { ...planWithConflicts([]), direction: "up" },
      onclose: vi.fn(),
      onconfirm,
    });

    expect(queryByText(/Conflicts \(/)).toBeNull();
    await fireEvent.click(getByRole("button", { name: "Start Sync" }));
    expect(onconfirm).toHaveBeenCalledWith([]);
  });
});
