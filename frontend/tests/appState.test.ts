import { describe, it, expect, beforeEach, vi } from "vitest";
import { appState } from "../src/lib/stores/appState.svelte";

const mockApi = vi.hoisted(() => ({ getStatus: vi.fn(), getRulesReport: vi.fn() }));
vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));
import type { orchestrator } from "../wailsjs/go/models";

describe("appState.svelte.ts", () => {
  beforeEach(() => {
    appState.statusPlan = null;
    appState.gitInfo = {
      isInstalled: true,
      isRepo: true,
      hasUpstream: true,
      branch: "main",
      remoteUrl: "https://example.com/repo.git",
      ahead: 0,
      behind: 0,
      userName: "Test User",
      userEmail: "test@example.com",
    };
  });

  it("handleStatusChanged replaces statusPlan and updates ahead/behind in gitInfo", () => {
    const newPlan: orchestrator.SyncPlan = {
      fingerprint: "fp-test-12345",
      conflicts: [{ path: "shared/AGENTS.md", reason: "remote_and_local_overlap" }],
      ahead: 3,
      behind: 2,
      direction: "conflict",
    };

    appState.handleStatusChanged(newPlan);

    expect(appState.statusPlan).toEqual(newPlan);
    expect(appState.statusPlan?.fingerprint).toBe("fp-test-12345");
    expect(appState.statusPlan?.conflicts.length).toBe(1);
    expect(appState.statusPlan?.direction).toBe("conflict");

    // gitInfo should have updated counts
    expect(appState.gitInfo?.ahead).toBe(3);
    expect(appState.gitInfo?.behind).toBe(2);
  });
});

describe("appState rule change refresh", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    appState.config = { rootPath: "C:/repo" } as any;
    appState.statusPlan = null;
    appState.isChecking = false;
  });

  it("loads the local plan without touching the checking state", async () => {
    mockApi.getStatus.mockResolvedValue({ rulesPending: 2 });
    const refresh = appState.refreshLocalStatus();
    expect(appState.isChecking).toBe(false);
    await refresh;
    expect(appState.statusPlan?.rulesPending).toBe(2);
  });

  it("skips the request when no root folder is set", async () => {
    appState.config = { rootPath: "" } as any;
    await appState.refreshLocalStatus();
    expect(mockApi.getStatus).not.toHaveBeenCalled();
  });

  it("keeps the newest plan when responses arrive out of order", async () => {
    let resolveFirst: (plan: unknown) => void = () => {};
    mockApi.getStatus
      .mockImplementationOnce(() => new Promise((resolve) => (resolveFirst = resolve)))
      .mockResolvedValueOnce({ rulesPending: 5 });
    const first = appState.refreshLocalStatus();
    await appState.refreshLocalStatus();
    resolveFirst({ rulesPending: 1 });
    await first;
    expect(appState.statusPlan?.rulesPending).toBe(5);
  });

  it("reports a failed refresh as an error notification", async () => {
    appState.alertNotifications = [];
    mockApi.getStatus.mockRejectedValue(new Error("boom"));
    await appState.refreshLocalStatus();
    expect(appState.alertNotifications.length).toBe(1);
    appState.alertNotifications = [];
  });

  it("afterRuleChange bumps the rules version and refreshes the plan", async () => {
    mockApi.getStatus.mockResolvedValue({ rulesPending: 1 });
    mockApi.getRulesReport.mockResolvedValue({ rules: [], collisions: [], unmapped: [] });
    const before = appState.rulesVersion;
    appState.afterRuleChange();
    expect(appState.rulesVersion).toBe(before + 1);
    await vi.waitFor(() => expect(appState.statusPlan?.rulesPending).toBe(1));
  });
});

describe("appState notices", () => {
  beforeEach(() => {
    appState.alertNotifications = [];
  });

  it("adds an informational bell notification and replaces the previous one of its kind", () => {
    appState.addNotice("auto_applied", "Updates applied", "2 files updated from Friend");
    appState.addNotice("auto_applied", "Updates applied", "1 file updated from Friend");
    appState.addNotice("other", "Other", "Something else");

    const messages = appState.alertNotifications.map((item) => item.message);
    expect(messages).toEqual(["1 file updated from Friend", "Something else"]);
    expect(appState.alertNotifications[0].variant).toBe("info");
  });
});
