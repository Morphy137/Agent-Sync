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
  openURL: vi.fn(),
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

  it("lists incoming files only after expanding the card's expand control", async () => {
    const { findByText, getByRole, queryByText, container } = render(SubscriptionsView);
    await findByText("Team skills");
    const expand = getByRole("button", { name: "1 incoming file" });
    expect(expand.getAttribute("aria-expanded")).toBe("false");
    expect(queryByText("skills/review/SKILL.md")).toBeNull();
    expect(container.querySelector(".zone-expand")).not.toBeNull();

    await fireEvent.click(expand);
    expect(expand.getAttribute("aria-expanded")).toBe("true");
    expect(getByRole("button", { name: "View changes in skills/review/SKILL.md" })).toBeDefined();
    await fireEvent.click(expand);
    expect(queryByText("skills/review/SKILL.md")).toBeNull();
  });

  it("reserves the expand zone on subscriptions without incoming changes", async () => {
    mockApi.listSubscriptions.mockResolvedValue([{ ...subscription, update: undefined }]);
    const { findByText, queryByRole, container } = render(SubscriptionsView);
    await findByText("Team skills");
    expect(queryByRole("button", { name: /incoming file/ })).toBeNull();
    expect(container.querySelector(".zone-expand")?.children.length).toBe(0);
  });

  it("opens the repository page from the branch button and hides the URL line", async () => {
    const { findByRole, queryByText } = render(SubscriptionsView);
    const branch = await findByRole("button", { name: /Open the repository page/ });
    expect(branch.getAttribute("title")).toBe(subscription.url);
    expect(branch.textContent).toContain("main");
    expect(queryByText(subscription.url)).toBeNull();
    await fireEvent.click(branch);
    expect(mockApi.openURL).toHaveBeenCalledWith("https://example.com/team/skills");
  });

  it("does not navigate for URLs without a web page", async () => {
    mockApi.listSubscriptions.mockResolvedValue([
      { ...subscription, url: "file:///C:/repos/skills" },
    ]);
    const { findByText, queryByRole, container } = render(SubscriptionsView);
    await findByText("Team skills");
    expect(queryByRole("button", { name: /Open the repository page/ })).toBeNull();
    expect(container.querySelector(".branch")?.getAttribute("title")).toBe(
      "file:///C:/repos/skills"
    );
  });

  it("shows the short hint and compact action labels with full tooltips", async () => {
    const { findByText, findByRole, getByText } = render(SubscriptionsView);
    await findByText("Team skills");
    expect(getByText("Local changes are kept.")).toBeDefined();
    const check = await findByRole("button", { name: "Check for updates" });
    expect(check.textContent?.trim()).toBe("Check");
    expect(check.parentElement?.getAttribute("title")).toBe("Check for updates");
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
    const { findByRole } = render(SubscriptionsView);

    await fireEvent.click(await findByRole("button", { name: "Update now" }));
    await waitFor(() => expect(mockApi.updateSubscription).toHaveBeenCalledWith("s_1"));
    await waitFor(() =>
      expect(addToast).toHaveBeenCalledWith(
        "success",
        "Team skills updated: 1 changed · 1 written."
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
