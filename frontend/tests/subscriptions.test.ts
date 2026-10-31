import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";
import { appState } from "../src/lib/stores/appState.svelte";

const mockApi = vi.hoisted(() => ({
  listSubscriptions: vi.fn(),
  addSubscription: vi.fn(),
  removeSubscription: vi.fn(),
  checkSubscription: vi.fn(),
  updateSubscription: vi.fn(),
  setSubscriptionAutoUpdate: vi.fn(),
  getSubscriptionTree: vi.fn(),
  getSubscriptionIncomingDiff: vi.fn(),
  openPath: vi.fn(),
}));

vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));

import SubscriptionsView from "../src/lib/views/SubscriptionsView.svelte";
import SubscriptionTreeDrawer from "../src/lib/components/subscriptions/SubscriptionTreeDrawer.svelte";

const subscription = {
  id: "s_1",
  name: "Team skills",
  url: "https://example.com/team/skills.git",
  branch: "main",
  autoUpdate: false,
  addedAt: "2026-10-30T10:00:00Z",
  lastCheckedAt: "2026-10-30T11:00:00Z",
  ruleCount: 1,
  update: {
    subscriptionId: "s_1",
    behind: 2,
    incomingFiles: ["skills/review/SKILL.md"],
    headCommit: "a",
    remoteCommit: "b",
    offline: false,
  },
};

describe("SubscriptionsView.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    mockApi.listSubscriptions.mockResolvedValue([subscription]);
  });

  afterEach(() => cleanup());

  it("renders subscriptions with their update badge", async () => {
    const { findByText, getByText } = render(SubscriptionsView);
    expect(await findByText("Team skills")).toBeDefined();
    expect(getByText("2 updates")).toBeDefined();
  });

  it("updates a subscription and reports the result", async () => {
    mockApi.updateSubscription.mockResolvedValue({
      changedFiles: ["skills/review/SKILL.md"],
      applied: {
        written: 1,
        captured: 0,
        deleted: 0,
        backups: [],
        skipped: 0,
        capturedRepoPaths: [],
      },
    });
    const addToast = vi.spyOn(appState, "addToast");
    const { findByText } = render(SubscriptionsView);

    await fireEvent.click(await findByText("Update now"));
    await waitFor(() => expect(mockApi.updateSubscription).toHaveBeenCalledWith("s_1"));
    await waitFor(() =>
      expect(addToast).toHaveBeenCalledWith(
        "success",
        "Team skills updated: 1 file(s) changed, 1 written."
      )
    );
  });

  it("adds a subscription from a URL", async () => {
    mockApi.listSubscriptions.mockResolvedValue([]);
    mockApi.addSubscription.mockResolvedValue({ ...subscription, update: undefined });
    const { findAllByText, findByLabelText, getByRole } = render(SubscriptionsView);

    await fireEvent.click((await findAllByText("Add subscription"))[0]);
    await fireEvent.input(await findByLabelText("Repository URL"), {
      target: { value: "https://example.com/team/skills.git" },
    });
    await fireEvent.click(getByRole("button", { name: "Add" }));
    await waitFor(() =>
      expect(mockApi.addSubscription).toHaveBeenCalledWith(
        "https://example.com/team/skills.git",
        ""
      )
    );
  });
});

describe("SubscriptionTreeDrawer.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    mockApi.getSubscriptionTree.mockResolvedValue([
      { path: "skills", isDir: true, size: 0, kind: "folder" },
      { path: "skills/review", isDir: true, size: 0, kind: "skill" },
      { path: "skills/review/SKILL.md", isDir: false, size: 20, kind: "file" },
    ]);
  });

  afterEach(() => {
    appState.closeRuleEditor();
    cleanup();
  });

  it("opens the rule editor with the picked subscription entry as source", async () => {
    const { findByText, findByRole, getByText } = render(SubscriptionTreeDrawer, {
      open: true,
      subscription: subscription as any,
      onclose: vi.fn(),
    });
    const expand = await findByRole("button", { name: "Expand skills" });
    if (expand.getAttribute("aria-expanded") !== "true") await fireEvent.click(expand);
    await fireEvent.click(await findByText("review"));
    await fireEvent.click(getByText("Sync this skill..."));

    expect(appState.ruleEditor.open).toBe(true);
    expect(appState.ruleEditor.scope).toBe("local");
    expect(appState.ruleEditor.presetSource).toMatchObject({
      origin: "subscription",
      subscription: "s_1",
      path: "skills/review",
    });
  });
});
