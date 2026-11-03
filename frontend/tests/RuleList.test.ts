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
const folderDestination = {
  id: "d1",
  location: "claude",
  path: "skills",
  mode: "two_way",
  enabled: true,
};

interface Options {
  multiple?: boolean;
  enabled?: boolean;
  files?: (typeof file)[];
  folder?: boolean;
  origin?: "repo" | "subscription";
}

function renderRule({
  multiple = false,
  enabled = true,
  files = [file],
  folder = false,
  origin = "repo",
}: Options = {}) {
  const ontoggle = vi.fn();
  const onedit = vi.fn();
  const onduplicate = vi.fn();
  const ondelete = vi.fn();
  const onopen = vi.fn();
  const onreview = vi.fn();
  const onsource = vi.fn();
  const onview = vi.fn();
  const first = folder ? folderDestination : firstDestination;
  const status = {
    rule: {
      id: "r1",
      name: folder ? "Skills" : "Agent instructions",
      enabled,
      source: {
        origin,
        path: folder ? "skills" : "shared/AGENTS.md",
        ...(origin === "subscription" ? { subscription: "s_1" } : {}),
      },
      kind: "auto",
      destinations: multiple ? [first, secondDestination] : [first],
    },
    scope: "repo",
    detectedKind: folder ? "folder" : "instructions",
    destinations: [
      {
        ruleId: "r1",
        destId: "d1",
        state: enabled ? "ready" : "disabled",
        targetRoot: "C:/home/.claude",
        files: enabled ? files : [],
      },
      ...(multiple
        ? [
            {
              ruleId: "r1",
              destId: "d2",
              state: enabled ? "blocked_collision" : "disabled",
              targetRoot: "",
              files: [],
            },
          ]
        : []),
    ],
  };
  return {
    ...render(RuleCard, {
      status: status as any,
      sourcePath,
      onsource,
      compact: true,
      ontoggle,
      onedit,
      onduplicate,
      ondelete,
      onopen,
      onview,
      onreview,
    }),
    ontoggle,
    onedit,
    onopen,
    onreview,
    onsource,
    onview,
  };
}

const zoneNames = (container: HTMLElement) =>
  [...container.querySelectorAll(".zone")].map((zone) =>
    [...zone.classList].find((name) => name.startsWith("zone-"))
  );

const zoneTexts = (container: HTMLElement) =>
  [...container.querySelectorAll(".zone")].map((zone) => zone.textContent?.trim());

describe("Compact rule list", () => {
  beforeEach(() => setLocale("en"));
  afterEach(() => cleanup());

  it("opens the source in the file editor", async () => {
    const { getByRole, onsource } = renderRule();
    await fireEvent.click(getByRole("button", { name: "AGENTS.md" }));
    expect(onsource).toHaveBeenCalledOnce();
  });

  it("shows the source, destination and readable status for a single file", async () => {
    const { getByRole, getByText, onreview } = renderRule();
    expect(getByRole("article", { name: "Agent instructions" })).toBeDefined();
    expect(getByText("Repo")).toBeDefined();
    expect(getByText("AGENTS.md").getAttribute("title")).toContain("Repository");
    expect(getByText("claude")).toBeDefined();
    expect(getByText("CLAUDE.md")).toBeDefined();
    expect(getByRole("img", { name: "Rule status: Changed on both sides" })).toBeDefined();
    await fireEvent.click(getByRole("button", { name: "Review CLAUDE.md" }));
    expect(onreview).toHaveBeenCalledWith("d1", file);
  });

  it("keeps a single-file rule collapsed: no expand control, but its zone stays reserved", () => {
    const { container, queryByRole } = renderRule();
    expect(queryByRole("button", { name: /Show details/ })).toBeNull();
    const zone = container.querySelector(".zone-expand");
    expect(zone).not.toBeNull();
    expect(zone!.children.length).toBe(0);
  });

  it("shows only the first destination in the row and folds the rest into the expand control", async () => {
    const { getByRole, getByText, queryByText, queryByRole } = renderRule({ multiple: true });
    expect(getByText("claude")).toBeDefined();
    expect(getByText("CLAUDE.md")).toBeDefined();
    expect(queryByText("gemini")).toBeNull();
    expect(queryByText("GEMINI.md")).toBeNull();

    const expand = getByRole("button", { name: "Show details for Agent instructions" });
    expect(expand.getAttribute("aria-expanded")).toBe("false");
    expect(expand.getAttribute("title")).toBe("2 destinations");
    expect(expand.textContent?.trim()).toBe("2");

    await fireEvent.click(expand);
    expect(expand.getAttribute("aria-expanded")).toBe("true");
    expect(getByRole("button", { name: "Hide details for Agent instructions" })).toBe(expand);
    expect(getByText("gemini")).toBeDefined();
    expect(getByText("GEMINI.md")).toBeDefined();
    expect(getByRole("img", { name: "Paused: overlap" })).toBeDefined();

    await fireEvent.click(expand);
    expect(queryByText("GEMINI.md")).toBeNull();
    expect(queryByRole("group", { name: "Destinations (2)" })).toBeNull();
  });

  it("lists the synced files of a folder only after expanding it", async () => {
    const synced = [
      { ...file, relPath: "a/SKILL.md", status: "in_sync" },
      { ...file, relPath: "b/SKILL.md", status: "conflict" },
    ];
    const { getByRole, queryByText, getByText, queryByRole, onreview } = renderRule({
      folder: true,
      files: synced,
    });
    const expand = getByRole("button", { name: "Show details for Skills" });
    expect(expand.textContent?.trim()).toBe("2");
    expect(expand.getAttribute("title")).toBe("2 synced files");
    expect(expand.getAttribute("aria-expanded")).toBe("false");
    expect(queryByText("a/SKILL.md")).toBeNull();
    expect(queryByRole("button", { name: "Review b/SKILL.md" })).toBeNull();

    await fireEvent.click(expand);
    expect(expand.getAttribute("aria-expanded")).toBe("true");
    expect(getByText("a/SKILL.md")).toBeDefined();
    expect(getByText("b/SKILL.md")).toBeDefined();
    await fireEvent.click(getByRole("button", { name: "Review b/SKILL.md" }));
    expect(onreview).toHaveBeenCalledWith("d1", synced[1]);
  });

  it("shows a conflict for a folder even when the destination is ready", () => {
    const synced = { ...file, relPath: "other.md", status: "in_sync" };
    const { getByRole } = renderRule({ folder: true, files: [synced, file] });
    expect(getByRole("img", { name: "Rule status: Changed on both sides" })).toBeDefined();
  });

  it("uses the same zones, in the same order, on every card", () => {
    const plain = renderRule();
    const plainZones = zoneNames(plain.container);
    cleanup();
    const folder = renderRule({ folder: true, multiple: true, origin: "subscription" });
    expect(plainZones).toEqual([
      "zone-name",
      "zone-origin",
      "zone-source",
      "zone-expand",
      "zone-chip",
      "zone-arrow",
      "zone-dest-chip",
      "zone-dest-file",
      "zone-mode",
      "zone-view",
      "zone-open",
      "zone-toggle",
      "zone-menu",
    ]);
    expect(zoneNames(folder.container)).toEqual(plainZones);
  });

  it("puts the subscription chip in its own zone and reserves it on other cards", () => {
    const subscription = renderRule({ origin: "subscription" });
    expect(subscription.container.querySelector(".zone-chip")?.textContent).toContain("s_1");
    cleanup();
    const repo = renderRule();
    expect(repo.container.querySelector(".zone-chip")?.textContent?.trim()).toBe("");
  });

  it("exposes the enabled state and forwards the toggle", async () => {
    const { getByRole, ontoggle, onopen } = renderRule({ enabled: false });
    await fireEvent.click(getByRole("button", { name: "Open claude destination" }));
    expect(onopen).toHaveBeenCalledWith("C:/home/.claude");
    const toggle = getByRole("switch", { name: "Rule enabled" });
    expect(toggle.getAttribute("aria-checked")).toBe("false");
    await fireEvent.click(toggle);
    expect(ontoggle).toHaveBeenCalledWith(true);
  });

  it("only dims a disabled rule: the same zones and expand control remain", () => {
    const enabled = renderRule({ folder: true, multiple: true });
    const zones = zoneNames(enabled.container);
    const enabledTexts = zoneTexts(enabled.container);
    expect(enabled.container.querySelector(".rule-disabled")).toBeNull();
    cleanup();

    const disabled = renderRule({ folder: true, multiple: true, enabled: false });
    expect(disabled.container.querySelector(".rule-card")!.classList).toContain("rule-disabled");
    expect(zoneNames(disabled.container)).toEqual(zones);
    expect(disabled.getByRole("button", { name: "Show details for Skills" })).toBeDefined();
    // The file count is unknown while a rule is off; nothing else changes.
    const changed = zoneTexts(disabled.container)
      .map((text, index) => (text === enabledTexts[index] ? null : zones[index]))
      .filter(Boolean);
    expect(changed.every((name) => name === "zone-expand")).toBe(true);
  });

  it("opens the viewer from an expanded file without triggering Review", async () => {
    const synced = [
      { ...file, relPath: "a/SKILL.md", status: "in_sync" },
      { ...file, relPath: "b/SKILL.md", status: "conflict" },
    ];
    const { getByRole, onview, onreview } = renderRule({ folder: true, files: synced });
    await fireEvent.click(getByRole("button", { name: "Show details for Skills" }));
    await fireEvent.click(getByRole("button", { name: "View b/SKILL.md" }));
    expect(onview).toHaveBeenCalledWith("b/SKILL.md");
    expect(onreview).not.toHaveBeenCalled();
    await fireEvent.click(getByRole("button", { name: "Review b/SKILL.md" }));
    expect(onreview).toHaveBeenCalledOnce();
    expect(onview).toHaveBeenCalledOnce();
  });

  it("does not offer to view a file that no longer exists in the source", async () => {
    const synced = [{ ...file, relPath: "gone.md", status: "source_deleted" }];
    const { getByRole, getByText, queryByRole } = renderRule({ folder: true, files: synced });
    await fireEvent.click(getByRole("button", { name: "Show details for Skills" }));
    expect(getByText("gone.md")).toBeDefined();
    expect(queryByRole("button", { name: "View gone.md" })).toBeNull();
  });

  it("offers the viewer button even when the rule is disabled", async () => {
    const { getByRole, onview } = renderRule({ enabled: false });
    await fireEvent.click(getByRole("button", { name: "View source of Agent instructions" }));
    expect(onview).toHaveBeenCalledWith();
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
