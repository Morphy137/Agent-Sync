import { describe, it, expect, vi, beforeEach } from "vitest";
import { setLocale } from "../src/lib/i18n";

const mockApi = vi.hoisted(() => ({
  getAppInfo: vi.fn(),
  checkForUpdate: vi.fn(),
  installUpdate: vi.fn(),
  openURL: vi.fn(),
}));

vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));

import { updateState } from "../src/lib/stores/updates.svelte";

const release = {
  version: "0.0.3",
  tag: "v0.0.3",
  pageUrl: "https://github.com/Morphy137/Agent-Sync/releases/tag/v0.0.3",
};

describe("updateState", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    updateState.status = null;
    updateState.info = null;
  });

  it("reports when a newer version exists on a manual check", async () => {
    mockApi.checkForUpdate.mockResolvedValue({ current: "0.0.2", available: true, release });
    const toast = vi.fn();
    await updateState.check(true, toast);
    expect(updateState.available).toBe(true);
    expect(toast).toHaveBeenCalledWith("info", "Version 0.0.3 is available");
  });

  it("says the app is up to date", async () => {
    mockApi.checkForUpdate.mockResolvedValue({ current: "0.0.2", available: false, release });
    const toast = vi.fn();
    await updateState.check(true, toast);
    expect(toast).toHaveBeenCalledWith("success", "You have the latest version (0.0.2).");
  });

  it("stays silent when a background check fails", async () => {
    mockApi.checkForUpdate.mockRejectedValue("[update_check_failed]");
    const toast = vi.fn();
    await updateState.check(false, toast);
    expect(toast).not.toHaveBeenCalled();
  });

  it("installs in place when installed, otherwise opens the release page", async () => {
    updateState.status = { current: "0.0.2", available: true, release } as any;
    updateState.info = { version: "0.0.2", installed: true, portable: false, dataDir: "" } as any;
    mockApi.installUpdate.mockResolvedValue(undefined);
    await updateState.install(vi.fn());
    expect(mockApi.installUpdate).toHaveBeenCalled();

    updateState.info = { version: "0.0.2", installed: false, portable: true, dataDir: "" } as any;
    await updateState.install(vi.fn());
    expect(mockApi.openURL).toHaveBeenCalledWith(release.pageUrl);
    expect(mockApi.installUpdate).toHaveBeenCalledTimes(1);
  });
});
