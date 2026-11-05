import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";
import { appState } from "../src/lib/stores/appState.svelte";
import { matchesRuleQuery } from "../src/lib/components/rules/ruleUi";

const mockApi = vi.hoisted(() => ({
  getRulesReport: vi.fn(),
  migrateRuleCategories: vi.fn(),
  getRuleCategories: vi.fn(),
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
    mockApi.migrateRuleCategories.mockResolvedValue(undefined);
    mockApi.getRuleCategories.mockResolvedValue([]);
    mockApi.getStatus.mockResolvedValue({ rulesPending: 0 });
  });

  afterEach(() => {
    cleanup();
    appState.ruleReport = null;
    appState.activeView = "rules";
    vi.restoreAllMocks();
    delete (document as any).elementFromPoint;
    localStorage.removeItem("agentsync_category_order:C:/repo");
    localStorage.removeItem("agentsync_rule_order:C:/repo");
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

  it("groups rules, preserves collapsed sections, and reveals search results", async () => {
    localStorage.removeItem("agentsync_collapsed_categories:C:/repo");
    mockApi.getRuleCategories.mockResolvedValue([
      { id: "instructions", name: "", color: "green", scope: "builtin" },
      { id: "skill", name: "", color: "purple", scope: "builtin" },
    ]);
    mockApi.getRulesReport.mockResolvedValue({
      rules: [
        { ...instructions, rule: { ...instructions.rule, categoryId: "instructions" } },
        { ...skills, rule: { ...skills.rule, categoryId: "skill" } },
      ],
      collisions: [],
      unmapped: [],
      pending: 0,
      conflicts: 0,
    });
    const view = render(RulesView);
    const section = await view.findByRole("button", { name: /Instructions.*1 rule/i });
    await fireEvent.click(section);
    expect(view.queryByRole("article", { name: "Agent instructions" })).toBeNull();
    expect(
      JSON.parse(localStorage.getItem("agentsync_collapsed_categories:C:/repo") ?? "[]")
    ).toContain("instructions");
    await fireEvent.input(await view.findByPlaceholderText("Search rules"), {
      target: { value: "agent" },
    });
    expect(await view.findByRole("article", { name: "Agent instructions" })).toBeDefined();
    localStorage.removeItem("agentsync_collapsed_categories:C:/repo");
  });

  async function setupReordering() {
    localStorage.removeItem("agentsync_category_order:C:/repo");
    localStorage.removeItem("agentsync_rule_order:C:/repo");
    mockApi.getRuleCategories.mockResolvedValue([
      { id: "instructions", name: "", color: "green", scope: "builtin" },
      { id: "skill", name: "", color: "purple", scope: "builtin" },
    ]);
    const other = makeStatus("r3", "Other skill", { origin: "repo", path: "other" }, []);
    mockApi.getRulesReport.mockResolvedValue({
      rules: [
        { ...instructions, rule: { ...instructions.rule, categoryId: "instructions" } },
        { ...skills, rule: { ...skills.rule, categoryId: "skill" } },
        { ...other, rule: { ...other.rule, categoryId: "skill" } },
      ],
      collisions: [],
      unmapped: [],
      pending: 0,
      conflicts: 0,
    });
    const view = render(RulesView);
    await view.findByRole("button", { name: /Drag to reorder Skills;/ });
    return view;
  }

  function pointerEvent(target: Element | Window, type: string, x: number, y: number) {
    const event = new MouseEvent(type, { bubbles: true, button: 0, clientX: x, clientY: y });
    Object.defineProperty(event, "pointerId", { value: 1 });
    return fireEvent(target, event);
  }

  function hit(target: Element | null) {
    Object.defineProperty(document, "elementFromPoint", {
      configurable: true,
      value: vi.fn(() => target),
    });
  }

  it("previews and commits a category drag on the placeholder", async () => {
    const view = await setupReordering();
    const handle = view.getByRole("button", { name: /Drag to reorder Instructions;/ });
    const source = handle.closest(".category") as HTMLElement;
    const target = view.container.querySelector('section[aria-label="Skills"]') as HTMLElement;
    await pointerEvent(handle, "pointerdown", 20, 20);
    await pointerEvent(window, "pointermove", 22, 22);
    expect(document.querySelector(".rule-drag-preview")).toBeNull();
    hit(target);
    await pointerEvent(window, "pointermove", 50, 50);
    const clone = document.querySelector(".rule-drag-preview") as HTMLElement;
    expect(clone).not.toBeNull();
    expect(clone.getAttribute("aria-hidden")).toBe("true");
    expect(clone.hasAttribute("inert")).toBe(true);
    expect(clone.style.getPropertyValue("--category-color")).toContain("--category-green");
    expect(clone.querySelector("[id]")).toBeNull();
    expect(clone.style.left).toBe("30px");
    expect(document.documentElement.classList.contains("rules-dragging")).toBe(true);
    expect(Number(source.style.order)).toBeGreaterThan(Number(target.style.order));
    expect(localStorage.getItem("agentsync_category_order:C:/repo")).toBeNull();
    hit(view.container.querySelector(".category-list"));
    await pointerEvent(window, "pointerup", 50, 50);
    expect(
      [...view.container.querySelectorAll("section.category")].map((node) =>
        node.getAttribute("aria-label")
      )
    ).toEqual(["Skills", "Instructions"]);
    expect(JSON.parse(localStorage.getItem("agentsync_category_order:C:/repo") ?? "[]")[0]).toBe(
      "skill"
    );
    expect(document.querySelector(".rule-drag-preview")).toBeNull();
    expect(document.documentElement.classList.contains("rules-dragging")).toBe(false);
  });

  it("retains keyboard ordering and cancels a rule drag on Escape or outside the viewport", async () => {
    const view = await setupReordering();
    await fireEvent.keyDown(view.getByRole("button", { name: /Drag to reorder Skills;/ }), {
      key: "ArrowUp",
    });
    expect(view.container.querySelector("section.category")?.getAttribute("aria-label")).toBe(
      "Skills"
    );
    const handle = view.getByRole("button", { name: /Drag to reorder Team skills;/ });
    const source = handle.closest(".rule-order-row") as HTMLElement;
    const target = view
      .getByRole("article", { name: "Other skill" })
      .closest(".rule-order-row") as HTMLElement;
    hit(target);
    await pointerEvent(handle, "pointerdown", 20, 20);
    await pointerEvent(window, "pointermove", 50, 50);
    const preview = document.querySelector(".rule-drag-preview--rule") as HTMLElement;
    expect(preview).not.toBeNull();
    expect(preview.style.getPropertyValue("--category-color")).toContain("--category-purple");
    expect(Number(source.style.order)).toBeGreaterThan(Number(target.style.order));
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(localStorage.getItem("agentsync_rule_order:C:/repo")).toBeNull();
    expect(document.querySelector(".rule-drag-preview")).toBeNull();
    expect(document.documentElement.classList.contains("rules-dragging")).toBe(false);
    await pointerEvent(handle, "pointerdown", 20, 20);
    await pointerEvent(window, "pointermove", 50, 50);
    await pointerEvent(window, "pointerup", -1, 50);
    expect(localStorage.getItem("agentsync_rule_order:C:/repo")).toBeNull();
    expect(document.querySelector(".rule-drag-preview")).toBeNull();
  });

  it("commits a rule drag and cleans up on pointer cancellation and unmount", async () => {
    const view = await setupReordering();
    const handle = view.getByRole("button", { name: /Drag to reorder Team skills;/ });
    const target = view
      .getByRole("article", { name: "Other skill" })
      .closest(".rule-order-row") as HTMLElement;
    hit(target);
    await pointerEvent(handle, "pointerdown", 20, 20);
    await pointerEvent(window, "pointermove", 50, 50);
    await pointerEvent(window, "pointerup", 50, 50);
    expect(
      JSON.parse(localStorage.getItem("agentsync_rule_order:C:/repo") ?? "[]").indexOf("r3")
    ).toBeLessThan(
      JSON.parse(localStorage.getItem("agentsync_rule_order:C:/repo") ?? "[]").indexOf("r2")
    );
    const saved = localStorage.getItem("agentsync_rule_order:C:/repo");
    await pointerEvent(handle, "pointerdown", 20, 20);
    await pointerEvent(window, "pointermove", 50, 50);
    await pointerEvent(window, "pointercancel", 50, 50);
    expect(localStorage.getItem("agentsync_rule_order:C:/repo")).toBe(saved);
    await pointerEvent(handle, "pointerdown", 20, 20);
    await pointerEvent(window, "pointermove", 50, 50);
    await pointerEvent(handle, "lostpointercapture", 50, 50);
    expect(document.querySelector(".rule-drag-preview")).toBeNull();
    expect(localStorage.getItem("agentsync_rule_order:C:/repo")).toBe(saved);
    await pointerEvent(handle, "pointerdown", 20, 20);
    await pointerEvent(window, "pointermove", 50, 50);
    view.unmount();
    expect(document.querySelector(".rule-drag-preview")).toBeNull();
    expect(document.documentElement.classList.contains("rules-dragging")).toBe(false);
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
