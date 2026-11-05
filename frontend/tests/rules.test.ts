import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";
import { appState } from "../src/lib/stores/appState.svelte";

const mockApi = vi.hoisted(() => ({
  getRulesReport: vi.fn(),
  getRuleCategories: vi.fn(),
  migrateRuleCategories: vi.fn(),
  saveRule: vi.fn(),
  deleteRule: vi.fn(),
  getStatus: vi.fn(),
  syncRules: vi.fn(),
  resolveRuleFile: vi.fn(),
  getRuleDiff: vi.fn(),
  listRepoTree: vi.fn(),
  listSubscriptions: vi.fn(),
  getSubscriptionTree: vi.fn(),
  detectSource: vi.fn(),
  suggestRuleCategory: vi.fn(),
  suggestPlacement: vi.fn(),
  listLocations: vi.fn(),
  importIntoRepo: vi.fn(),
  openPath: vi.fn(),
  selectFile: vi.fn(),
  selectFolder: vi.fn(),
}));

vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));

import RulesView from "../src/lib/views/RulesView.svelte";
import RuleEditorModal from "../src/lib/components/rules/RuleEditorModal.svelte";

const instructionsRule = {
  id: "r_1",
  name: "Agent instructions",
  enabled: true,
  source: { origin: "repo", path: "shared/AGENTS.md" },
  kind: "auto",
  destinations: [
    { id: "d_1", location: "claude", path: "CLAUDE.md", mode: "two_way", enabled: true },
    { id: "d_2", location: "gemini", path: "GEMINI.md", mode: "two_way", enabled: true },
  ],
};

function sampleReport() {
  return {
    rules: [
      {
        rule: instructionsRule,
        scope: "repo",
        detectedKind: "instructions",
        destinations: [
          {
            ruleId: "r_1",
            destId: "d_1",
            state: "ready",
            targetRoot: "C:/home/.claude",
            files: [
              {
                relPath: "CLAUDE.md",
                sourceAbs: "C:/repo/shared/AGENTS.md",
                targetAbs: "C:/home/.claude/CLAUDE.md",
                status: "conflict",
              },
            ],
          },
          { ruleId: "r_1", destId: "d_2", state: "blocked_collision", targetRoot: "", files: [] },
        ],
      },
    ],
    collisions: [
      {
        type: "same_target",
        path: "C:/home/.gemini/GEMINI.md",
        refs: [{ ruleId: "r_1", destId: "d_2" }],
      },
    ],
    unmapped: ["obsidian"],
    pending: 1,
    conflicts: 1,
  };
}

const locations = [
  {
    alias: "home",
    path: "C:/home",
    builtin: true,
    exists: true,
    profileId: "",
    profileLabel: "",
    ruleCount: 0,
  },
  {
    alias: "claude",
    path: "C:/home/.claude",
    builtin: false,
    exists: true,
    profileId: "claude-code",
    profileLabel: "Claude Code",
    ruleCount: 1,
  },
];

describe("RulesView.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    appState.config = { rootPath: "C:/repo" } as any;
    mockApi.getRulesReport.mockResolvedValue(sampleReport());
    mockApi.getStatus.mockResolvedValue({ rulesPending: 1 });
    mockApi.getRuleCategories.mockResolvedValue([
      { id: "skill", name: "", scope: "builtin", color: "purple" },
      { id: "instructions", name: "", scope: "builtin", color: "green" },
    ]);
  });

  afterEach(() => {
    cleanup();
    appState.statusPlan = null;
    appState.ruleReport = null;
    appState.collisionDetailsOpen = false;
    appState.alertNotifications = [];
    appState.fileDirty = false;
    appState.activeView = "rules";
    vi.restoreAllMocks();
  });

  it("keeps warnings in notifications and opens collision details on demand", async () => {
    const { findAllByText, queryByText, getByText } = render(RulesView);
    expect((await findAllByText("Agent instructions")).length).toBeGreaterThan(0);
    expect(queryByText("1 collision detected")).toBeNull();
    expect(queryByText("1 location not configured on this computer")).toBeNull();
    expect(appState.ruleNotifications.some((item) => item.id.startsWith("rule_unmapped:"))).toBe(
      true
    );
    const alert = appState.ruleNotifications.find((item) => item.id.startsWith("rule_collisions:"));
    expect(alert).toBeDefined();
    alert?.actions?.[0].onclick();
    await waitFor(() => expect(getByText("Same destination file")).toBeDefined());
    expect(getByText("C:/home/.gemini/GEMINI.md")).toBeDefined();
    await fireEvent.click(getByText("Edit Agent instructions"));
    expect(appState.ruleEditor.open).toBe(true);
  });

  it("keeps resolved warnings in the notification history without stale actions", () => {
    appState.setRuleReport(sampleReport() as any);
    appState.setRuleReport({ ...sampleReport(), collisions: [], unmapped: [] } as any);
    const items = appState.ruleNotifications;
    expect(items.map((item) => item.title)).toContain("1 collision detected · Resolved");
    expect(items.map((item) => item.title)).toContain(
      "1 location not configured on this computer · Resolved"
    );
    expect(items.every((item) => !item.actions)).toBe(true);
  });

  it("does not navigate from unsaved files when reviewing a warning is cancelled", () => {
    appState.setRuleReport(sampleReport() as any);
    appState.activeView = "files";
    appState.fileDirty = true;
    vi.spyOn(window, "confirm").mockReturnValue(false);
    appState.ruleNotifications
      .find((item) => item.id.startsWith("rule_collisions:"))
      ?.actions?.[0].onclick();
    expect(appState.activeView).toBe("files");
    expect(appState.fileDirty).toBe(true);
    expect(appState.collisionDetailsOpen).toBe(false);
  });

  it("records report errors once without retrying recursively", async () => {
    mockApi.getRulesReport.mockRejectedValue("report failed");
    await appState.refreshRuleReport();
    await appState.refreshRuleReport();
    expect(mockApi.getRulesReport).toHaveBeenCalledTimes(2);
    expect(appState.alertNotifications).toHaveLength(1);
  });

  it("records skipped synchronization files as a warning", async () => {
    mockApi.syncRules.mockResolvedValue({ written: 1, captured: 0, skipped: 2 });
    const { findByRole, getByRole } = render(RulesView);
    await findByRole("heading", { name: "Agent instructions" });
    await fireEvent.click(getByRole("button", { name: "Sync rules" }));
    await waitFor(() =>
      expect(
        appState.alertNotifications.some((item) => item.message.includes("2 need review"))
      ).toBe(true)
    );
  });

  it("uses distinct active notification ids for different repositories", () => {
    appState.setRuleReport(sampleReport() as any);
    const first = appState.ruleNotifications.find((item) =>
      item.id.startsWith("rule_collisions:")
    )?.id;
    appState.config = { rootPath: "C:/other" } as any;
    appState.setRuleReport(sampleReport() as any);
    const second = appState.ruleNotifications.find((item) =>
      item.id.startsWith("rule_collisions:")
    )?.id;
    expect(first).not.toBe(second);
    expect(appState.alertNotifications).toHaveLength(0);
  });

  it("always shows the compact list", async () => {
    const { findByRole, queryByRole } = render(RulesView);
    await findByRole("heading", { name: "Agent instructions" });
    expect(queryByRole("button", { name: "Show destinations for Agent instructions" })).toBeNull();
    expect(queryByRole("button", { name: "List view" })).toBeNull();
    expect(queryByRole("button", { name: "Grid view" })).toBeNull();
  });

  it("toggling a rule saves it with the new enabled flag", async () => {
    mockApi.saveRule.mockResolvedValue({ rule: instructionsRule, collisions: [] });
    const { findByRole } = render(RulesView);
    await fireEvent.click(await findByRole("switch", { name: "Rule enabled" }));
    await waitFor(() => expect(mockApi.saveRule).toHaveBeenCalled());
    const [payload, scope] = mockApi.saveRule.mock.calls[0];
    expect(payload.enabled).toBe(false);
    expect(scope).toBe("repo");
    await waitFor(() => expect(mockApi.getStatus).toHaveBeenCalled());
  });

  it("deleting a rule refreshes the pending state", async () => {
    mockApi.deleteRule.mockResolvedValue(undefined);
    const { findByRole } = render(RulesView);
    await fireEvent.click(
      await findByRole("button", { name: "More options for Agent instructions" })
    );
    await fireEvent.click(await findByRole("menuitem", { name: "Delete" }));
    await fireEvent.click(await findByRole("button", { name: "Delete rule" }));
    await waitFor(() => expect(mockApi.deleteRule).toHaveBeenCalledWith("r_1"));
    await waitFor(() => expect(mockApi.getStatus).toHaveBeenCalled());
    await waitFor(() => expect(appState.statusPlan?.rulesPending).toBe(1));
  });

  it("reviews a conflicting file and resolves it with the source version", async () => {
    mockApi.getRuleDiff.mockResolvedValue({
      leftLabel: "source",
      rightLabel: "destination",
      left: "a\n",
      right: "b\n",
      binary: false,
      transform: false,
    });
    mockApi.resolveRuleFile.mockResolvedValue(undefined);
    const { findByRole, findByText } = render(RulesView);

    await fireEvent.click(await findByRole("button", { name: "Review CLAUDE.md" }));
    expect(mockApi.getRuleDiff).toHaveBeenCalledWith("r_1", "d_1", "CLAUDE.md");

    await fireEvent.click(await findByText("Use source"));
    await waitFor(() => expect(mockApi.resolveRuleFile).toHaveBeenCalled());
    expect(mockApi.resolveRuleFile.mock.calls[0][0]).toMatchObject({
      ruleId: "r_1",
      destId: "d_1",
      relPath: "CLAUDE.md",
      choice: "use_source",
    });
  });
});

describe("RuleEditorModal.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    appState.config = { rootPath: "C:/repo" } as any;
    mockApi.getStatus.mockResolvedValue({ rulesPending: 1 });
    mockApi.getRuleCategories.mockResolvedValue([
      { id: "skill", name: "", scope: "builtin", color: "purple" },
      { id: "instructions", name: "", scope: "builtin", color: "green" },
    ]);
    mockApi.listLocations.mockResolvedValue(locations);
    mockApi.listRepoTree.mockResolvedValue([
      { path: "shared/AGENTS.md", isDir: false, size: 10, kind: "instructions" },
    ]);
    mockApi.listSubscriptions.mockResolvedValue([]);
    mockApi.suggestRuleCategory.mockResolvedValue("instructions");
    mockApi.detectSource.mockResolvedValue({
      kind: "instructions",
      kindId: "instructions",
      name: "AGENTS",
      baseName: "AGENTS.md",
      isDir: false,
    });
    mockApi.suggestPlacement.mockResolvedValue({
      profileId: "claude-code",
      label: "Claude Code",
      location: "claude",
      path: "CLAUDE.md",
      mode: "two_way",
      detected: true,
    });
  });

  afterEach(() => {
    appState.closeRuleEditor();
    cleanup();
  });

  it("defaults new rules to local while keeping shared selection and existing scope", async () => {
    appState.openRuleEditor();
    const view = render(RuleEditorModal);
    expect(view.getByRole("radio", { name: "Local" }).getAttribute("aria-checked")).toBe("true");
    await fireEvent.click(view.getByRole("radio", { name: "Shared (repository)" }));
    expect(
      view.getByRole("radio", { name: "Shared (repository)" }).getAttribute("aria-checked")
    ).toBe("true");

    appState.openRuleEditor({ rule: instructionsRule as any, scope: "repo" });
    await waitFor(() =>
      expect(
        view.getByRole("radio", { name: "Shared (repository)" }).getAttribute("aria-checked")
      ).toBe("true")
    );
  });

  it("preserves the scope of an existing shared subscription rule", () => {
    appState.openRuleEditor({
      rule: {
        ...instructionsRule,
        source: { origin: "subscription", subscription: "s_1", path: "AGENTS.md" },
      } as any,
      scope: "repo",
    });
    const view = render(RuleEditorModal);
    expect(
      view.getByRole("radio", { name: "Shared (repository)" }).getAttribute("aria-checked")
    ).toBe("true");
  });

  async function openWithPreset() {
    appState.openRuleEditor({
      presetSource: { origin: "repo", path: "shared/AGENTS.md" } as any,
      scope: "repo",
    });
    const view = render(RuleEditorModal);
    await waitFor(() => expect(mockApi.detectSource).toHaveBeenCalled());
    await fireEvent.click(await view.findByRole("combobox", { name: /^Location/ }));
    await fireEvent.click(await view.findByRole("option", { name: /claude/ }));
    await view.findByText("Detected: Claude Code");
    return view;
  }

  it("applies the detected placement and saves the rule", async () => {
    mockApi.saveRule.mockResolvedValue({ rule: instructionsRule, collisions: [] });
    const { getByDisplayValue, getByText } = await openWithPreset();

    expect(getByDisplayValue("CLAUDE.md")).toBeDefined();
    expect(getByDisplayValue("AGENTS.md")).toBeDefined();

    await fireEvent.click(getByText("Save rule"));
    await waitFor(() => expect(mockApi.saveRule).toHaveBeenCalled());
    const [payload, scope] = mockApi.saveRule.mock.calls[0];
    expect(scope).toBe("repo");
    expect(payload.categoryId).toBe("instructions");
    expect(payload.source).toMatchObject({ origin: "repo", path: "shared/AGENTS.md" });
    expect(payload.destinations[0]).toMatchObject({
      location: "claude",
      path: "CLAUDE.md",
      mode: "two_way",
    });
    await waitFor(() => expect(appState.ruleEditor.open).toBe(false));
    await waitFor(() => expect(mockApi.getStatus).toHaveBeenCalled());
    await waitFor(() => expect(appState.statusPlan?.rulesPending).toBe(1));
  });

  it("keeps a manually selected category when detection finishes", async () => {
    let finishDetection!: (value: any) => void;
    mockApi.suggestRuleCategory.mockReturnValue(
      new Promise((resolve) => {
        finishDetection = resolve;
      })
    );
    mockApi.saveRule.mockResolvedValue({ rule: instructionsRule, collisions: [] });
    appState.openRuleEditor({
      presetSource: { origin: "repo", path: "shared/AGENTS.md" } as any,
      scope: "repo",
    });
    const view = render(RuleEditorModal);
    await fireEvent.click(await view.findByRole("combobox", { name: /^Category/ }));
    await fireEvent.click(await view.findByRole("option", { name: "Skills" }));
    finishDetection("instructions");
    await fireEvent.click(await view.findByRole("combobox", { name: /^Location/ }));
    await fireEvent.click(await view.findByRole("option", { name: /claude/ }));
    await view.findByText("Detected: Claude Code");
    await fireEvent.click(view.getByText("Save rule"));
    await waitFor(() => expect(mockApi.saveRule).toHaveBeenCalled());
    expect(mockApi.saveRule.mock.calls[0][0].categoryId).toBe("skill");
  });

  it("keeps an explicit Uncategorized choice when detection finishes", async () => {
    let finishDetection!: (value: any) => void;
    mockApi.suggestRuleCategory.mockReturnValue(
      new Promise((resolve) => {
        finishDetection = resolve;
      })
    );
    mockApi.saveRule.mockResolvedValue({ rule: instructionsRule, collisions: [] });
    appState.openRuleEditor({
      presetSource: { origin: "repo", path: "shared/AGENTS.md" } as any,
      scope: "repo",
    });
    const view = render(RuleEditorModal);
    await fireEvent.click(await view.findByRole("combobox", { name: /^Category/ }));
    await fireEvent.click(await view.findByRole("option", { name: "Uncategorized" }));
    finishDetection("instructions");
    await fireEvent.click(await view.findByRole("combobox", { name: /^Location/ }));
    await fireEvent.click(await view.findByRole("option", { name: /claude/ }));
    await view.findByText("Detected: Claude Code");
    await fireEvent.click(view.getByText("Save rule"));
    await waitFor(() => expect(mockApi.saveRule).toHaveBeenCalled());
    expect(mockApi.saveRule.mock.calls[0][0].categoryId).toBe("uncategorized");
  });

  it("does not suggest a deleted built-in category", async () => {
    mockApi.getRuleCategories.mockResolvedValue([
      { id: "skill", name: "", scope: "builtin", color: "purple" },
    ]);
    mockApi.suggestRuleCategory.mockResolvedValue("uncategorized");
    mockApi.saveRule.mockResolvedValue({ rule: instructionsRule, collisions: [] });
    const view = await openWithPreset();
    await fireEvent.click(view.getByText("Save rule"));
    await waitFor(() => expect(mockApi.saveRule).toHaveBeenCalled());
    expect(mockApi.saveRule.mock.calls[0][0].categoryId).toBe("uncategorized");
  });

  it("warns about collisions after saving", async () => {
    mockApi.saveRule.mockResolvedValue({
      rule: instructionsRule,
      collisions: [
        {
          type: "same_target",
          path: "C:/home/.claude/CLAUDE.md",
          refs: [{ ruleId: "r_1", destId: "d_1" }],
        },
      ],
    });
    mockApi.getRulesReport.mockResolvedValue(sampleReport());
    const { getByText } = await openWithPreset();

    await fireEvent.click(getByText("Save rule"));
    await waitFor(() => expect(appState.ruleEditor.open).toBe(false));
    await waitFor(() =>
      expect(
        appState.ruleNotifications.some((item) => item.id.startsWith("rule_collisions:"))
      ).toBe(true)
    );
  });

  it("validates required fields before saving", async () => {
    appState.openRuleEditor({ scope: "repo" });
    const { findByText, getByText } = render(RuleEditorModal);
    await fireEvent.click(getByText("Save rule"));
    expect(await findByText("Choose a source file or folder.")).toBeDefined();
    expect(mockApi.saveRule).not.toHaveBeenCalled();
  });
});
