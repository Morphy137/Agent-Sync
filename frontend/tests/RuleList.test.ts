import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render } from "@testing-library/svelte";
import RuleCard from "../src/lib/components/rules/RuleCard.svelte";
import { setLocale } from "../src/lib/i18n";

const sourcePath = "C:/repo/shared/AGENTS.md";
const file = {
  relPath: "CLAUDE.md",
  sourceAbs: sourcePath,
  targetAbs: "C:/home/.claude/CLAUDE.md",
  status: "conflict",
};
const firstDestination = {
  id: "d1",
  location: "claude",
  path: "CLAUDE.md",
  mode: "two_way",
  enabled: true,
};
const secondDestination = {
  id: "d2",
  location: "gemini",
  path: "GEMINI.md",
  mode: "one_way",
  enabled: true,
};

function renderRule(multiple = false, enabled = true, files = [file]) {
  const ontoggle = vi.fn();
  const onedit = vi.fn();
  const onduplicate = vi.fn();
  const ondelete = vi.fn();
  const onopen = vi.fn();
  const onreview = vi.fn();
  const status = {
    rule: {
      id: "r1",
      name: "Agent instructions",
      enabled,
      source: { origin: "repo", path: "shared/AGENTS.md" },
      kind: "auto",
      destinations: multiple ? [firstDestination, secondDestination] : [firstDestination],
    },
    scope: "repo",
    detectedKind: "instructions",
    destinations: [
      { ruleId: "r1", destId: "d1", state: "ready", targetRoot: "C:/home/.claude", files },
      ...(multiple
        ? [{ ruleId: "r1", destId: "d2", state: "blocked_collision", targetRoot: "", files: [] }]
        : []),
    ],
  };
  return {
    ...render(RuleCard, {
      status: status as any,
      sourcePath,
      compact: true,
      ontoggle,
      onedit,
      onduplicate,
      ondelete,
      onopen,
      onreview,
    }),
    ontoggle,
    onedit,
    onopen,
    onreview,
  };
}

describe("Compact rule list", () => {
  beforeEach(() => setLocale("en"));
  afterEach(() => cleanup());

  it("shows the source, destination and readable status for a single file", async () => {
    const { getByRole, getByText, onreview } = renderRule();
    expect(getByRole("article", { name: "Agent instructions" })).toBeDefined();
    expect(getByText("Repo")).toBeDefined();
    expect(getByText("AGENTS.md").getAttribute("title")).toContain("Repository");
    expect(getByText("AGENTS.md")).toBeDefined();
    expect(getByText("claude")).toBeDefined();
    expect(getByText("CLAUDE.md")).toBeDefined();
    expect(getByRole("img", { name: "Changed on both sides" })).toBeDefined();
    await fireEvent.click(getByRole("button", { name: "Review CLAUDE.md" }));
    expect(onreview).toHaveBeenCalledWith("d1", file);
  });

  it("shows each destination and its status when a rule has several", () => {
    const { getByRole, getByText } = renderRule(true);
    expect(getByText("claude")).toBeDefined();
    expect(getByText("gemini")).toBeDefined();
    expect(getByText("GEMINI.md")).toBeDefined();
    expect(getByRole("img", { name: "Changed on both sides" })).toBeDefined();
    expect(getByRole("img", { name: "Paused: overlap" })).toBeDefined();
  });

  it("shows a conflict for a folder even when the destination is ready", () => {
    const synced = { ...file, relPath: "other.md", status: "in_sync" };
    const { getByRole } = renderRule(false, true, [synced, file]);
    expect(getByRole("img", { name: "Changed on both sides" })).toBeDefined();
    expect(getByRole("button", { name: "Review CLAUDE.md" })).toBeDefined();
  });

  it("exposes the enabled state and forwards the toggle", async () => {
    const { getByRole, ontoggle } = renderRule(false, false);
    const toggle = getByRole("switch", { name: "Rule enabled" });
    expect(toggle.getAttribute("aria-checked")).toBe("false");
    await fireEvent.click(toggle);
    expect(ontoggle).toHaveBeenCalledWith(true);
  });

  it("exposes the actions in an accessible menu", async () => {
    const { getByRole, queryByRole, onedit, onopen } = renderRule();
    const menuButton = getByRole("button", { name: "More options for Agent instructions" });
    expect(menuButton.getAttribute("aria-expanded")).toBe("false");
    await fireEvent.click(menuButton);
    expect(menuButton.getAttribute("aria-expanded")).toBe("true");
    expect(getByRole("menuitem", { name: "Duplicate" })).toBeDefined();
    expect(getByRole("menuitem", { name: "Delete" })).toBeDefined();
    await fireEvent.click(getByRole("menuitem", { name: "Open source" }));
    expect(onopen).toHaveBeenCalledWith(sourcePath);
    expect(queryByRole("menu")).toBeNull();
    await fireEvent.click(menuButton);
    await fireEvent.click(getByRole("menuitem", { name: "Edit" }));
    expect(onedit).toHaveBeenCalledOnce();
  });
});
