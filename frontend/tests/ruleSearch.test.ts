import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";
import { appState } from "../src/lib/stores/appState.svelte";
import { matchesRuleQuery } from "../src/lib/components/rules/ruleUi";

const mockApi = vi.hoisted(() => ({
  getRulesReport: vi.fn(),
  getStatus: vi.fn(),
  openPath: vi.fn(),
}));

vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));

import RulesView from "../src/lib/views/RulesView.svelte";

function makeStatus(
  id: string,
  name: string,
  source: Record<string, unknown>,
  destinations: { location: string; path: string }[]
) {
  return {
    rule: {
      id,
      name,
      enabled: true,
      source,
      kind: "auto",
      destinations: destinations.map((d, index) => ({
        id: `${id}_d${index}`,
        mode: "two_way",
        enabled: true,
        ...d,
      })),
    },
    scope: "repo",
    destinations: [],
  } as any;
}

const instructions = makeStatus(
  "r1",
  "Agent instructions",
  { origin: "repo", path: "shared/AGENTS.md" },
  [{ location: "claude", path: "CLAUDE.md" }]
);
const skills = makeStatus(
  "r2",
  "Team skills",
  { origin: "subscription", subscription: "s_1", path: "skills" },
  [{ location: "gemini", path: "skills/team" }]
);
const labels = { origin: "Repository", subscription: "" };

describe("matchesRuleQuery", () => {
  it("accepts everything for a blank query", () => {
    expect(matchesRuleQuery(instructions, "", labels)).toBe(true);
    expect(matchesRuleQuery(instructions, "   ", labels)).toBe(true);
  });

  it("matches name, source, origin and destination case-insensitively", () => {
    expect(matchesRuleQuery(instructions, "  AGENT inst ", labels)).toBe(true);
    expect(matchesRuleQuery(instructions, "shared/agents", labels)).toBe(true);
    expect(matchesRuleQuery(instructions, "repository", labels)).toBe(true);
    expect(matchesRuleQuery(instructions, "CLAUDE", labels)).toBe(true);
    expect(matchesRuleQuery(instructions, "claude/claude.md", labels)).toBe(true);
    expect(matchesRuleQuery(instructions, "gemini", labels)).toBe(false);
  });

  it("matches the subscription id and name", () => {
    const subscription = { origin: "Subscription", subscription: "Shared skills" };
    expect(matchesRuleQuery(skills, "s_1", subscription)).toBe(true);
    expect(matchesRuleQuery(skills, "shared sk", subscription)).toBe(true);
    expect(matchesRuleQuery(skills, "subscr", subscription)).toBe(true);
  });

  it("matches selected files of the source", () => {
    const selection = makeStatus(
      "r3",
      "Notes",
      { origin: "local", path: "C:/n", include: ["a.txt"] },
      []
    );
    expect(matchesRuleQuery(selection, "a.txt", labels)).toBe(true);
  });
});

describe("Rules search", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    appState.config = {
      rootPath: "C:/repo",
      subscriptions: [{ id: "s_1", name: "Shared" }],
    } as any;
    mockApi.getRulesReport.mockResolvedValue({
      rules: [instructions, skills],
      collisions: [],
      unmapped: [],
      pending: 0,
      conflicts: 0,
    });
    mockApi.getStatus.mockResolvedValue({ rulesPending: 0 });
  });

  afterEach(() => {
    cleanup();
    appState.ruleReport = null;
    appState.activeView = "rules";
    vi.restoreAllMocks();
  });

  it("filters as you type and shows the shown-of-total count", async () => {
    const { findByPlaceholderText, queryByRole, getByText } = render(RulesView);
    const field = await findByPlaceholderText("Search rules");
    expect(queryByRole("article", { name: "Team skills" })).not.toBeNull();
    await fireEvent.input(field, { target: { value: "  TEAM " } });
    await waitFor(() => expect(queryByRole("article", { name: "Agent instructions" })).toBeNull());
    expect(queryByRole("article", { name: "Team skills" })).not.toBeNull();
    expect(getByText(/1 of 2/)).toBeDefined();
  });

  it("shows an empty state with a clear action", async () => {
    const { findByPlaceholderText, getByRole, findByText, queryByRole } = render(RulesView);
    const field = await findByPlaceholderText("Search rules");
    await fireEvent.input(field, { target: { value: "zzz" } });
    expect(await findByText("No rules match")).toBeDefined();
    await fireEvent.click(getByRole("button", { name: "Show all rules" }));
    await waitFor(() => expect(queryByRole("article", { name: "Team skills" })).not.toBeNull());
    expect((field as HTMLInputElement).value).toBe("");
  });

  it("clears with the x button and with Escape, and keeps expanded cards", async () => {
    const { findByPlaceholderText, getByRole, queryByRole } = render(RulesView);
    const field = (await findByPlaceholderText("Search rules")) as HTMLInputElement;
    expect(queryByRole("button", { name: "Clear search" })).toBeNull();
    await fireEvent.input(field, { target: { value: "agent" } });
    await fireEvent.click(getByRole("button", { name: "Clear search" }));
    await waitFor(() => expect(field.value).toBe(""));
    await fireEvent.input(field, { target: { value: "agent" } });
    await fireEvent.keyDown(field, { key: "Escape" });
    await waitFor(() => expect(field.value).toBe(""));
    expect(queryByRole("article", { name: "Team skills" })).not.toBeNull();
  });

  it("hides the search when there are no rules", async () => {
    mockApi.getRulesReport.mockResolvedValue({
      rules: [],
      collisions: [],
      unmapped: [],
      pending: 0,
      conflicts: 0,
    });
    const { findByText, queryByPlaceholderText } = render(RulesView);
    await findByText("Write once, sync everywhere");
    expect(queryByPlaceholderText("Search rules")).toBeNull();
  });
});
