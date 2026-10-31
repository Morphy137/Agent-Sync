import { describe, it, expect, beforeEach } from "vitest";
import { appState } from "../src/lib/stores/appState.svelte";
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
      downloads: [{ profile: "CLAUDE", relPath: "CLAUDE.md", action: "create" }],
      uploads: [{ profile: "GEMINI", relPath: "GEMINI.md", action: "upload" }],
      conflicts: [
        {
          profile: "CODEX",
          relPath: "config.json",
          repoAction: "modified",
          targetAction: "modified",
          reason: "diverged",
        },
      ],
      ahead: 3,
      behind: 2,
      direction: "conflict",
      connections: [],
    };

    appState.handleStatusChanged(newPlan);

    expect(appState.statusPlan).toEqual(newPlan);
    expect(appState.statusPlan?.fingerprint).toBe("fp-test-12345");
    expect(appState.statusPlan?.downloads.length).toBe(1);
    expect(appState.statusPlan?.uploads.length).toBe(1);
    expect(appState.statusPlan?.conflicts.length).toBe(1);
    expect(appState.statusPlan?.direction).toBe("conflict");

    // gitInfo should have updated counts
    expect(appState.gitInfo?.ahead).toBe(3);
    expect(appState.gitInfo?.behind).toBe(2);
  });
});
