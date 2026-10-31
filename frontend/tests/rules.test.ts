import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";
import { appState } from "../src/lib/stores/appState.svelte";

const mockApi = vi.hoisted(() => ({
  getRulesReport: vi.fn(),
  saveRule: vi.fn(),
  deleteRule: vi.fn(),
  syncRules: vi.fn(),
  resolveRuleFile: vi.fn(),
  getRuleDiff: vi.fn(),
  listRepoTree: vi.fn(),
  listSubscriptions: vi.fn(),
  getSubscriptionTree: vi.fn(),
  detectSource: vi.fn(),
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
  });

  afterEach(() => cleanup());

  it("renders rules with collision and unmapped location banners", async () => {
    const { findAllByText, getByText } = render(RulesView);
    expect((await findAllByText("Agent instructions")).length).toBeGreaterThan(0);
    expect(getByText("1 collision(s) detected")).toBeDefined();
    expect(getByText("1 location(s) not set on this PC")).toBeDefined();
  });

  it("toggling a rule saves it with the new enabled flag", async () => {
    mockApi.saveRule.mockResolvedValue({ rule: instructionsRule, collisions: [] });
    const { findByRole } = render(RulesView);
    await fireEvent.click(await findByRole("switch", { name: "Rule enabled" }));
    await waitFor(() => expect(mockApi.saveRule).toHaveBeenCalled());
    const [payload, scope] = mockApi.saveRule.mock.calls[0];
    expect(payload.enabled).toBe(false);
    expect(scope).toBe("repo");
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
    mockApi.listLocations.mockResolvedValue(locations);
    mockApi.listRepoTree.mockResolvedValue([
      { path: "shared/AGENTS.md", isDir: false, size: 10, kind: "instructions" },
    ]);
    mockApi.listSubscriptions.mockResolvedValue([]);
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

  async function openWithPreset() {
    appState.openRuleEditor({
      presetSource: { origin: "repo", path: "shared/AGENTS.md" } as any,
      scope: "repo",
    });
    const view = render(RuleEditorModal);
    await waitFor(() => expect(mockApi.detectSource).toHaveBeenCalled());
    await fireEvent.change(await view.findByLabelText("Location"), {
      target: { value: "claude" },
    });
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
    expect(payload.source).toMatchObject({ origin: "repo", path: "shared/AGENTS.md" });
    expect(payload.destinations[0]).toMatchObject({
      location: "claude",
      path: "CLAUDE.md",
      mode: "two_way",
    });
    await waitFor(() => expect(appState.ruleEditor.open).toBe(false));
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
    const { getByText, findByText } = await openWithPreset();

    await fireEvent.click(getByText("Save rule"));
    expect(await findByText("Rule saved with collisions")).toBeDefined();
    expect(appState.ruleEditor.open).toBe(true);
  });

  it("validates required fields before saving", async () => {
    appState.openRuleEditor({ scope: "repo" });
    const { findByText, getByText } = render(RuleEditorModal);
    await fireEvent.click(getByText("Save rule"));
    expect(await findByText("Choose a source file or folder.")).toBeDefined();
    expect(mockApi.saveRule).not.toHaveBeenCalled();
  });
});
