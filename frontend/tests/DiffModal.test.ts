import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, cleanup } from "@testing-library/svelte";
import DiffModal from "../src/lib/components/DiffModal.svelte";
import { buildLineDiff } from "../src/lib/diff/lineDiff";
import { setLocale } from "../src/lib/i18n";

const sampleDiff = {
  leftLabel: "source",
  rightLabel: "destination",
  left: "a\nb\nc\n",
  right: "a\nB\nc\nd\n",
  binary: false,
  transform: false,
};

describe("buildLineDiff", () => {
  it("numbers both sides and counts changes", () => {
    const summary = buildLineDiff(sampleDiff.left, sampleDiff.right);
    expect(summary.added).toBe(2);
    expect(summary.removed).toBe(1);
    expect(summary.rows[0]).toEqual({ kind: "context", oldLine: 1, newLine: 1, text: "a" });
    expect(summary.rows.find((r) => r.kind === "removed")?.text).toBe("b");
  });

  it("reports identical content", () => {
    expect(buildLineDiff("x\n", "x\n").identical).toBe(true);
  });
});

describe("DiffModal.svelte", () => {
  beforeEach(() => setLocale("en"));
  afterEach(() => cleanup());

  it("renders translated labels and runs actions", async () => {
    const onUse = vi.fn();
    const { getByText } = render(DiffModal, {
      open: true,
      diff: sampleDiff,
      actions: [{ label: "Use source", onclick: onUse }],
      onclose: vi.fn(),
    });
    expect(getByText("Left: Source · Right: Destination")).toBeDefined();
    expect(getByText("2 added · 1 removed")).toBeDefined();
    await fireEvent.click(getByText("Use source"));
    expect(onUse).toHaveBeenCalled();
  });

  it("shows a notice for binary files", () => {
    const { getByText } = render(DiffModal, {
      open: true,
      diff: { ...sampleDiff, binary: true },
      onclose: vi.fn(),
    });
    expect(getByText(/can't be compared/)).toBeDefined();
  });

  it("lets the user edit and save a merged version", async () => {
    const onSave = vi.fn();
    const { getByText, getByLabelText } = render(DiffModal, {
      open: true,
      diff: sampleDiff,
      onsaveEdited: onSave,
      onclose: vi.fn(),
    });
    await fireEvent.click(getByText("Edit merged version"));
    const area = getByLabelText("Merged content") as HTMLTextAreaElement;
    await fireEvent.input(area, { target: { value: "merged\n" } });
    await fireEvent.click(getByText("Save edited version"));
    expect(onSave).toHaveBeenCalledWith("merged\n");
  });
});
