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

function renderRule(count: number, mixed = false) {
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
          files: index === 0 || (count === 5 && index === 3) ? [file] : [],
        })),
      } as any,
      sourcePath: "C:/repo/AGENTS.md",
      ontoggle: vi.fn(),
      onedit: vi.fn(),
      onduplicate: vi.fn(),
      ondelete: vi.fn(),
      onopen,
      onreview,
    }),
    onopen,
    onreview,
  };
}

describe("Rule grid destination summary", () => {
  beforeEach(() => setLocale("en"));
  afterEach(() => cleanup());

  it("allows expanding a single destination to review and open it", async () => {
    const { getByRole, queryByRole, onopen, onreview } = renderRule(1);
    const chip = getByRole("img", { name: "claude: Changed on both sides" });
    expect(getByRole("img", { name: "Rule status: Changed on both sides" })).toBeDefined();
    expect(chip.getAttribute("title")).toContain("CLAUDE.md");
    expect(queryByRole("button", { name: "Review CLAUDE.md" })).toBeNull();
    const expand = getByRole("button", { name: "Show destinations for Agent instructions" });
    expect(expand.getAttribute("aria-expanded")).toBe("false");
    await fireEvent.click(expand);
    expect(
      getByRole("button", { name: "Hide destinations for Agent instructions" }).getAttribute(
        "aria-expanded"
      )
    ).toBe("true");
    await fireEvent.click(getByRole("button", { name: "Review CLAUDE.md" }));
    expect(onreview).toHaveBeenCalledWith("d1", file);
    await fireEvent.click(getByRole("button", { name: "Open claude destination" }));
    expect(onopen).toHaveBeenCalledWith("C:/home/claude");
    await fireEvent.click(
      getByRole("button", { name: "Hide destinations for Agent instructions" })
    );
    expect(queryByRole("button", { name: "Review CLAUDE.md" })).toBeNull();
  });

  it("summarizes three destinations and reveals all five on expansion", async () => {
    const { getByRole, getByText, queryByText, queryByRole } = renderRule(5);
    expect(getByRole("img", { name: "claude: Changed on both sides" })).toBeDefined();
    const blocked = getByRole("img", { name: "gemini: Paused: overlap" });
    expect(blocked.getAttribute("title")).toContain("gemini/instructions.md");
    expect(getByRole("img", { name: "codex: Ready" })).toBeDefined();
    expect(queryByRole("img", { name: "cursor: Ready" })).toBeNull();
    expect(
      getByRole("img", { name: "cursor/cursor/instructions.md: Changed on both sides" })
    ).toBeDefined();
    expect(getByText("+2")).toBeDefined();
    expect(queryByText("cursor/instructions.md")).toBeNull();
    await fireEvent.click(
      getByRole("button", { name: "Show destinations for Agent instructions" })
    );
    expect(getByText("cursor/instructions.md")).toBeDefined();
    expect(getByText("other/instructions.md")).toBeDefined();
    await fireEvent.click(
      getByRole("button", { name: "Hide destinations for Agent instructions" })
    );
    expect(queryByText("other/instructions.md")).toBeNull();
    expect(getByText("+2")).toBeDefined();
  });

  it("keeps individual modes visible when destinations use different modes", async () => {
    const { getByRole, getByText, queryByText } = renderRule(2, true);
    expect(queryByText("Two-way")).toBeNull();
    await fireEvent.click(
      getByRole("button", { name: "Show destinations for Agent instructions" })
    );
    expect(getByText("Two-way")).toBeDefined();
    expect(getByText("One-way")).toBeDefined();
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
