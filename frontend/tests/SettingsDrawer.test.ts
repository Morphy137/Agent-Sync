import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import SettingsDrawer from "../src/lib/components/SettingsDrawer.svelte";
import { appState, SETTINGS_DEFAULTS } from "../src/lib/stores/appState.svelte";
import { updateState } from "../src/lib/stores/updates.svelte";
import { setLocale, getActiveLocale } from "../src/lib/i18n";
import { applyTheme } from "../src/lib/theme/theme";

const validFolder = vi.hoisted(() => ({
  normalized: "",
  exists: true,
  isDir: true,
  writable: true,
  isInsideRoot: false,
  alreadyConnected: false,
}));

vi.mock("../wailsjs/go/main/App", () => ({
  InspectPath: vi.fn().mockResolvedValue(validFolder),
  SelectFolder: vi.fn(),
  CreateFolder: vi.fn(),
  GetChangelog: vi.fn().mockResolvedValue("## [0.0.1] - 2026-10-31\n### Added\n- First release\n"),
}));

const mockApi = vi.hoisted(() => ({
  updateSettings: vi.fn(),
  setGitIdentity: vi.fn(),
  getGitInfo: vi.fn(),
  getStartupStatus: vi.fn(),
  getRuleCategories: vi.fn(),
  setRootPath: vi.fn(),
  openURL: vi.fn(),
}));
vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));

/** Longer than the autosave debounce so a pending save has fired. */
const PAST_DEBOUNCE_MS = 1500;

function savedConfig(overrides: Record<string, unknown> = {}) {
  return {
    language: "en",
    theme: "light",
    backupPath: "C:/backups",
    backupRetention: 20,
    checkIntervalMinutes: 5,
    confirmBeforeSync: true,
    disableUpdateCheck: false,
    rootPath: "C:/repo/root",
    tutorialCompleted: true,
    ...overrides,
  };
}

describe("SettingsDrawer.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    applyTheme("light");

    appState.config = savedConfig() as any;
    appState.gitInfo = {
      isInstalled: true,
      isRepo: true,
      userName: "Initial User",
      userEmail: "user@example.com",
    } as any;
    appState.alertNotifications = [];
    updateState.info = { dataDir: "C:\\Users\\me\\AppData\\Roaming\\AgentSync" } as any;

    mockApi.updateSettings.mockImplementation(async (payload) => ({
      ...(appState.config as any),
      ...payload,
    }));
    mockApi.getGitInfo.mockResolvedValue({ isRepo: true, userName: "x", userEmail: "x@x.com" });
    mockApi.getStartupStatus.mockResolvedValue({ supported: true, enabled: false });
    mockApi.getRuleCategories.mockResolvedValue([]);
  });

  afterEach(() => {
    cleanup();
    updateState.info = null;
  });

  function renderDrawer(onclose = vi.fn()) {
    return { onclose, ...render(SettingsDrawer, { open: true, onclose }) };
  }

  it("has no save button", () => {
    const { queryByRole, getByRole } = renderDrawer();
    expect(queryByRole("button", { name: /save settings/i })).toBeNull();
    expect(getByRole("button", { name: "Restore defaults" })).toBeDefined();
  });

  it("saves language and theme right away and keeps them applied", async () => {
    const { getByText } = renderDrawer();

    await fireEvent.click(getByText("Español"));
    await waitFor(() =>
      expect(mockApi.updateSettings).toHaveBeenCalledWith(
        expect.objectContaining({ language: "es", theme: "light" })
      )
    );
    expect(getActiveLocale()).toBe("es");
    await waitFor(() => expect(appState.config?.language).toBe("es"));

    await fireEvent.click(getByText("Negro"));
    await waitFor(() =>
      expect(mockApi.updateSettings).toHaveBeenLastCalledWith(
        expect.objectContaining({ language: "es", theme: "dark" })
      )
    );
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(appState.toasts).toHaveLength(0);
  });

  it("saves a toggle immediately", async () => {
    const { getByRole } = renderDrawer();
    const confirmSwitch = getByRole("switch", { name: "Confirm before syncing" });
    await fireEvent.click(confirmSwitch);
    await waitFor(() =>
      expect(mockApi.updateSettings).toHaveBeenCalledWith(
        expect.objectContaining({ confirmBeforeSync: false })
      )
    );
  });

  it("saves the retention after the debounce, not on every keystroke", async () => {
    const { getByLabelText } = renderDrawer();
    const retention = getByLabelText("Number of backups to keep");

    await fireEvent.input(retention, { target: { value: "3" } });
    await fireEvent.input(retention, { target: { value: "30" } });
    expect(mockApi.updateSettings).not.toHaveBeenCalled();

    await waitFor(() => expect(mockApi.updateSettings).toHaveBeenCalledTimes(1), {
      timeout: PAST_DEBOUNCE_MS,
    });
    expect(mockApi.updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ backupRetention: 30 })
    );
  });

  it("flushes a pending edit when the drawer closes", async () => {
    const { getByLabelText, getByRole, onclose } = renderDrawer();
    await fireEvent.input(getByLabelText("Number of backups to keep"), {
      target: { value: "7" },
    });
    expect(mockApi.updateSettings).not.toHaveBeenCalled();

    await fireEvent.click(getByRole("button", { name: "Close" }));
    expect(onclose).toHaveBeenCalled();
    await waitFor(() =>
      expect(mockApi.updateSettings).toHaveBeenCalledWith(
        expect.objectContaining({ backupRetention: 7 })
      )
    );
  });

  it("does not save an invalid retention and shows the inline message", async () => {
    const { getByLabelText, findByRole, getByRole } = renderDrawer();
    await fireEvent.input(getByLabelText("Number of backups to keep"), {
      target: { value: "0" },
    });
    expect((await findByRole("alert")).textContent).toContain("whole number");

    // Closing flushes nothing and another setting still keeps the saved retention.
    await fireEvent.click(getByRole("button", { name: "Close" }));
    await new Promise((resolve) => setTimeout(resolve, PAST_DEBOUNCE_MS));
    expect(mockApi.updateSettings).not.toHaveBeenCalled();
  });

  it("never overwrites with an invalid value when another setting changes", async () => {
    const { getByLabelText, getByRole } = renderDrawer();
    await fireEvent.input(getByLabelText("Number of backups to keep"), {
      target: { value: "-4" },
    });
    await fireEvent.click(getByRole("switch", { name: "Confirm before syncing" }));
    await waitFor(() => expect(mockApi.updateSettings).toHaveBeenCalled());
    expect(mockApi.updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ backupRetention: 20, confirmBeforeSync: false })
    );
  });

  it("reports a failed save and restores the last saved values", async () => {
    mockApi.updateSettings.mockRejectedValueOnce(new Error("disk full"));
    const { getByText } = renderDrawer();

    await fireEvent.click(getByText("Español"));
    await waitFor(() => expect(appState.alertNotifications.length).toBe(1));
    await waitFor(() => expect(getActiveLocale()).toBe("en"));
    expect(appState.config?.language).toBe("en");
  });

  it("serializes saves so an older response cannot overwrite a newer edit", async () => {
    const resolvers: Array<() => void> = [];
    mockApi.updateSettings.mockImplementation(
      (payload) =>
        new Promise((resolve) => {
          resolvers.push(() => resolve({ ...(appState.config as any), ...payload }));
        })
    );
    const { getByText } = renderDrawer();

    await fireEvent.click(getByText("Español"));
    await fireEvent.click(getByText("Negro"));
    await waitFor(() => expect(resolvers).toHaveLength(1));
    expect(mockApi.updateSettings).toHaveBeenCalledTimes(1);

    resolvers[0]();
    await waitFor(() => expect(resolvers).toHaveLength(2));
    resolvers[1]();
    await waitFor(() => expect(appState.config?.theme).toBe("dark"));
    expect(appState.config?.language).toBe("es");
  });

  describe("restore defaults", () => {
    it("asks for confirmation and sends nothing until confirmed", async () => {
      const { getByRole, findByRole } = renderDrawer();
      await fireEvent.click(getByRole("button", { name: "Restore defaults" }));
      expect(await findByRole("dialog", { name: "Restore default settings?" })).toBeDefined();
      expect(mockApi.updateSettings).not.toHaveBeenCalled();

      await fireEvent.click(getByRole("button", { name: "Cancel" }));
      expect(mockApi.updateSettings).not.toHaveBeenCalled();
    });

    it("sends only the drawer settings with their default values", async () => {
      appState.config = savedConfig({
        language: "es",
        theme: "dark",
        backupRetention: 3,
        checkIntervalMinutes: 30,
        confirmBeforeSync: false,
        disableUpdateCheck: true,
        backupPath: "D:/elsewhere",
        autoApplyUpdates: true,
        closeToTray: true,
        startWithWindows: true,
      }) as any;
      mockApi.getStartupStatus.mockResolvedValue({ supported: true, enabled: true });
      const { getByRole } = renderDrawer();

      await fireEvent.click(getByRole("button", { name: "Restore defaults" }));
      await fireEvent.click(getByRole("button", { name: "Restore" }));

      await waitFor(() => expect(mockApi.updateSettings).toHaveBeenCalledTimes(1));
      expect(mockApi.updateSettings).toHaveBeenCalledWith({
        language: SETTINGS_DEFAULTS.language,
        theme: SETTINGS_DEFAULTS.theme,
        backupPath: "C:\\Users\\me\\AppData\\Roaming\\AgentSync\\backups",
        backupRetention: SETTINGS_DEFAULTS.backupRetention,
        checkIntervalMinutes: SETTINGS_DEFAULTS.checkIntervalMinutes,
        confirmBeforeSync: SETTINGS_DEFAULTS.confirmBeforeSync,
        disableUpdateCheck: SETTINGS_DEFAULTS.disableUpdateCheck,
        autoApplyUpdates: false,
        closeToTray: false,
        startWithWindows: false,
      });
      expect(mockApi.setRootPath).not.toHaveBeenCalled();
      expect(mockApi.setGitIdentity).not.toHaveBeenCalled();
      await waitFor(() => expect(appState.config?.rootPath).toBe("C:/repo/root"));
    });
  });

  describe("background options", () => {
    const AUTO_APPLY = "Apply subscription updates automatically";
    const CLOSE_TO_TRAY = "Keep running in the system tray";
    const START_WITH_WINDOWS = "Start with Windows";

    it("are off by default and shown in their own section", () => {
      const { getByRole, getByText } = renderDrawer();
      expect(getByText("Background")).toBeDefined();
      for (const name of [AUTO_APPLY, CLOSE_TO_TRAY, START_WITH_WINDOWS]) {
        expect(getByRole("switch", { name }).getAttribute("aria-checked")).toBe("false");
      }
    });

    it("save immediately with the rest of the settings", async () => {
      const { getByRole } = renderDrawer();

      await fireEvent.click(getByRole("switch", { name: AUTO_APPLY }));
      await waitFor(() =>
        expect(mockApi.updateSettings).toHaveBeenLastCalledWith(
          expect.objectContaining({ autoApplyUpdates: true, closeToTray: false })
        )
      );
      await fireEvent.click(getByRole("switch", { name: CLOSE_TO_TRAY }));
      await waitFor(() =>
        expect(mockApi.updateSettings).toHaveBeenLastCalledWith(
          expect.objectContaining({ autoApplyUpdates: true, closeToTray: true })
        )
      );
      await fireEvent.click(getByRole("switch", { name: START_WITH_WINDOWS }));
      await waitFor(() =>
        expect(mockApi.updateSettings).toHaveBeenLastCalledWith(
          expect.objectContaining({ startWithWindows: true })
        )
      );
    });

    it("show the real startup state, not the stored wish", async () => {
      appState.config = savedConfig({ startWithWindows: true }) as any;
      mockApi.getStartupStatus.mockResolvedValue({ supported: true, enabled: false });
      const { getByRole } = renderDrawer();

      await waitFor(() =>
        expect(getByRole("switch", { name: START_WITH_WINDOWS }).getAttribute("aria-checked")).toBe(
          "false"
        )
      );
      // Nothing to save: the entry already matches what the switch shows.
      await fireEvent.click(getByRole("switch", { name: AUTO_APPLY }));
      await waitFor(() => expect(mockApi.updateSettings).toHaveBeenCalledTimes(1));
      expect(mockApi.updateSettings).toHaveBeenCalledWith(
        expect.objectContaining({ startWithWindows: false })
      );
    });

    it("disable the startup switch in development builds", async () => {
      mockApi.getStartupStatus.mockResolvedValue({ supported: false, enabled: false });
      const { getByRole, findByText } = renderDrawer();

      expect(await findByText("Not available in development builds.")).toBeDefined();
      const startup = getByRole("switch", { name: START_WITH_WINDOWS }) as HTMLButtonElement;
      expect(startup.disabled).toBe(true);
      await fireEvent.click(startup);
      await new Promise((resolve) => setTimeout(resolve, 50));
      expect(mockApi.updateSettings).not.toHaveBeenCalled();
    });

    it("explain what the startup entry does with the tray on or off", async () => {
      const { getByRole, getByText } = renderDrawer();
      expect(getByText("Starts minimized when you sign in.")).toBeDefined();
      await fireEvent.click(getByRole("switch", { name: CLOSE_TO_TRAY }));
      expect(getByText("Starts hidden in the tray when you sign in.")).toBeDefined();
    });
  });

  it("opens the public GitHub repository from the star button", async () => {
    const { getByRole } = renderDrawer();
    await fireEvent.click(getByRole("button", { name: "Star on GitHub" }));
    expect(mockApi.openURL).toHaveBeenCalledWith("https://github.com/Morphy137/Agent-Sync");
  });
});
