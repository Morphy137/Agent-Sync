import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render } from "@testing-library/svelte";
import RuleCard from "../src/lib/components/rules/RuleCard.svelte";
import { setLocale } from "../src/lib/i18n";
import { destinationSummary } from "../src/lib/components/rules/ruleUi";

const file = {
  relPath: "CLAUDE.md",
  sourceAbs: "C:/repo/AGENTS.md",
  targetAbs: "C:/home/claude/CLAUDE.md",
  status: "conflict",
};

function renderRule(count: number, mixed = false, fileStatus = file.status) {
  const names = ["claude", "gemini", "codex", "cursor", "other"];
  const destinations = names.slice(0, count).map((location, index) => ({
    id: `d${index + 1}`,
    location,
    path: index === 0 ? "CLAUDE.md" : `${location}/instructions.md`,
    mode: mixed && index === 1 ? "one_way" : "two_way",
    enabled: true,
  }));
  const onopen = vi.fn();
  const onreview = vi.fn();
  const onsource = vi.fn();
  const onview = vi.fn();
  return {
    ...render(RuleCard, {
      status: {
        rule: {
          id: "r1",
          name: "Agent instructions",
          enabled: true,
          source: { origin: "repo", path: "AGENTS.md" },
          kind: "auto",
          destinations,
        },
        scope: "repo",
        detectedKind: "instructions",
        destinations: destinations.map((destination, index) => ({
          ruleId: "r1",
          destId: destination.id,
          state: index === 1 ? "blocked_collision" : "ready",
          targetRoot: `C:/home/${destination.location}`,
          files:
            index === 0 || (count === 5 && index === 3) ? [{ ...file, status: fileStatus }] : [],
        })),
      } as any,
      sourcePath: "C:/repo/AGENTS.md",
      onsource,
      ontoggle: vi.fn(),
      onedit: vi.fn(),
      onduplicate: vi.fn(),
      ondelete: vi.fn(),
      onopen,
      onview,
      onreview,
    }),
    onopen,
    onreview,
    onsource,
    onview,
  };
}

describe("Rule card destinations", () => {
  beforeEach(() => setLocale("en"));
  afterEach(() => cleanup());

  it("opens the source in the file editor", async () => {
    const { getByRole, onsource } = renderRule(1);
    await fireEvent.click(getByRole("button", { name: "AGENTS.md" }));
    expect(onsource).toHaveBeenCalledOnce();
  });

  it("reviews and opens a single destination straight from the row, without an expand control", async () => {
    const { getByRole, getByText, queryByRole, onopen, onreview } = renderRule(1);
    expect(getByRole("img", { name: "Rule status: Changed on both sides" })).toBeDefined();
    expect(getByText("claude").getAttribute("title")).toContain("claude/CLAUDE.md");
    expect(queryByRole("button", { name: /Show details/ })).toBeNull();
    await fireEvent.click(getByRole("button", { name: "Review CLAUDE.md" }));
    expect(onreview).toHaveBeenCalledWith("d1", file);
    await fireEvent.click(getByRole("button", { name: "Open claude destination" }));
    expect(onopen).toHaveBeenCalledWith("C:/home/claude");
  });

  it("summarizes five destinations in one row and reveals all of them on expansion", async () => {
    const { container, getByRole, getByText, queryByText } = renderRule(5);
    expect(getByText("claude")).toBeDefined();
    expect(queryByText("gemini")).toBeNull();
    expect(queryByText("cursor/instructions.md")).toBeNull();
    const expand = getByRole("button", { name: "Show details for Agent instructions" });
    expect(expand.textContent?.trim()).toBe("5");
    expect(expand.getAttribute("title")).toBe("5 destinations");
    // A hidden destination needs attention, so the collapsed control says so.
    expect(container.querySelector(".expand-attention")).not.toBeNull();

    await fireEvent.click(expand);
    expect(getByRole("button", { name: "Hide details for Agent instructions" })).toBe(expand);
    expect(getByRole("img", { name: "Paused: overlap" })).toBeDefined();
    expect(getByText("gemini")).toBeDefined();
    expect(getByText("cursor/instructions.md")).toBeDefined();
    expect(getByText("other/instructions.md")).toBeDefined();

    await fireEvent.click(expand);
    expect(queryByText("other/instructions.md")).toBeNull();
    expect(expand.getAttribute("aria-expanded")).toBe("false");
  });

  it("keeps individual modes out of the row when destinations use different modes", async () => {
    const { getByRole, getByText, queryByText } = renderRule(2, true, "in_sync");
    expect(queryByText("Two-way")).toBeNull();
    await fireEvent.click(getByRole("button", { name: "Show details for Agent instructions" }));
    expect(getByText("Two-way")).toBeDefined();
    expect(getByText("One-way")).toBeDefined();
  });

  it("shows the shared mode on the row", () => {
    const { getByText, queryByRole } = renderRule(2, false, "in_sync");
    expect(getByText("Two-way")).toBeDefined();
    expect(queryByRole("button", { name: "Review CLAUDE.md" })).toBeNull();
  });

  it("shows Review in place of the mode on a row that needs review", () => {
    const { container, getByRole, queryByText } = renderRule(2);
    expect(queryByText("Two-way")).toBeNull();
    const zone = container.querySelector(".zone-mode")!;
    expect(zone.contains(getByRole("button", { name: "Review CLAUDE.md" }))).toBe(true);
  });

  it("opens the read-only source viewer from the row button", async () => {
    const { getByRole, onview } = renderRule(1);
    await fireEvent.click(getByRole("button", { name: "View source of Agent instructions" }));
    expect(onview).toHaveBeenCalledWith();
  });
});

describe("destination status summary", () => {
  it("prioritizes conflicts in folder contents and does not mark missing status as synced", () => {
    const rule = { enabled: true } as any;
    const destination = { enabled: true } as any;
    const status = {
      state: "ready",
      files: [{ status: "in_sync" }, { status: "conflict" }],
    } as any;
    expect(destinationSummary(rule, destination, status)).toEqual({
      tone: "conflict",
      labelKey: "rules.file_status.conflict",
    });
    expect(destinationSummary(rule, destination, undefined)).toEqual({
      tone: "idle",
      labelKey: "rules.dest_state.unknown",
    });
    expect(
      destinationSummary(rule, destination, {
        state: "ready",
        files: [{ status: "in_sync" }],
      } as any)
    ).toEqual({ tone: "ok", labelKey: "rules.file_status.in_sync" });
  });
});
