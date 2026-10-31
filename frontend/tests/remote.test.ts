import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";
import { appState } from "../src/lib/stores/appState.svelte";

const mockApi = vi.hoisted(() => ({
  connectRemote: vi.fn(),
  getRepoFileDiff: vi.fn(),
  discardRepoFile: vi.fn(),
  getConfig: vi.fn(),
  listProfiles: vi.fn(),
  getGitInfo: vi.fn(),
  getStatus: vi.fn(),
}));

vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));

import ConnectRemoteModal from "../src/lib/components/remote/ConnectRemoteModal.svelte";
import RepoChangesModal from "../src/lib/components/remote/RepoChangesModal.svelte";

function stubRefresh() {
  mockApi.getConfig.mockResolvedValue({ rootPath: "C:/repo" });
  mockApi.listProfiles.mockResolvedValue([]);
  mockApi.getGitInfo.mockResolvedValue({ isRepo: true, hasUpstream: true });
  mockApi.getStatus.mockResolvedValue(appState.statusPlan);
}

describe("ConnectRemoteModal.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    appState.gitInfo = { isRepo: false } as any;
    stubRefresh();
  });

  afterEach(() => cleanup());

  it("connects and shows the summary with the backup branch and local changes", async () => {
    mockApi.connectRemote.mockResolvedValue({
      branch: "main",
      remoteHadContent: true,
      backupBranch: "agentsync/local-backup-20261030-120000",
      localChanges: [{ status: "M", path: "shared/AGENTS.md" }],
    });
    const onreview = vi.fn();
    const { findByLabelText, getByRole, findByText } = render(ConnectRemoteModal, {
      open: true,
      onclose: vi.fn(),
      onreview,
    });

    await fireEvent.input(await findByLabelText("Repository URL"), {
      target: { value: "https://github.com/me/agent-configs.git" },
    });
    await fireEvent.click(getByRole("button", { name: "Connect" }));

    await waitFor(() =>
      expect(mockApi.connectRemote).toHaveBeenCalledWith("https://github.com/me/agent-configs.git")
    );
    expect(await findByText("Branch: main")).toBeDefined();
    expect(await findByText("agentsync/local-backup-20261030-120000")).toBeDefined();
    await fireEvent.click(getByRole("button", { name: "Review changes" }));
    expect(onreview).toHaveBeenCalled();
  });
});

describe("RepoChangesModal.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    appState.statusPlan = {
      fingerprint: "f",
      direction: "up",
      downloads: [],
      uploads: [],
      conflicts: [],
      commitPreview: ["shared/AGENTS.md"],
      ahead: 0,
      behind: 0,
      hasRemote: true,
      isOffline: false,
      dirtyOutside: [],
      dirtyOverlapExists: false,
      connections: [],
      rulesPending: 0,
      rulesConflicts: 0,
    } as any;
    stubRefresh();
  });

  afterEach(() => cleanup());

  it("takes the repository version of a local change after confirmation", async () => {
    mockApi.discardRepoFile.mockResolvedValue(undefined);
    const { findByRole, getByRole } = render(RepoChangesModal, { open: true, onclose: vi.fn() });

    await fireEvent.click(
      await findByRole("button", { name: "Take the repository version of shared/AGENTS.md" })
    );
    await fireEvent.click(getByRole("button", { name: "Replace my file" }));
    await waitFor(() => expect(mockApi.discardRepoFile).toHaveBeenCalledWith("shared/AGENTS.md"));
  });

  it("opens the diff of a local change", async () => {
    mockApi.getRepoFileDiff.mockResolvedValue({
      leftLabel: "repository",
      rightLabel: "local",
      left: "a\n",
      right: "b\n",
      binary: false,
      transform: false,
    });
    const { findByRole, findByText } = render(RepoChangesModal, { open: true, onclose: vi.fn() });

    await fireEvent.click(await findByRole("button", { name: "View changes in shared/AGENTS.md" }));
    await waitFor(() =>
      expect(mockApi.getRepoFileDiff).toHaveBeenCalledWith("shared/AGENTS.md", "working")
    );
    expect(await findByText("Left: Repository · Right: This PC")).toBeDefined();
  });
});
