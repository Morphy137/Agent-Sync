import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, cleanup } from "@testing-library/svelte";
import SettingsDrawer from "../src/lib/components/SettingsDrawer.svelte";
import { appState } from "../src/lib/stores/appState.svelte";
import { setLocale, getActiveLocale } from "../src/lib/i18n";
import { applyTheme } from "../src/lib/theme/theme";

vi.mock("../wailsjs/go/main/App", () => ({
  InspectPath: vi.fn(),
  SelectFolder: vi.fn(),
  CreateFolder: vi.fn(),
}));

describe("SettingsDrawer.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    applyTheme("light");

    appState.config = {
      language: "en",
      theme: "light",
      backupPath: "C:/backups",
      backupRetention: 20,
      checkIntervalMinutes: 5,
      confirmBeforeSync: true,
      rootPath: "C:/repo/root",
      connections: [],
    } as any;

    appState.gitInfo = {
      isInstalled: true,
      isRepo: true,
      userName: "Initial User",
      userEmail: "user@example.com",
    } as any;
  });

  afterEach(() => {
    cleanup();
  });

  it("reverts unsaved theme and language changes on close without saving (R5)", async () => {
    const closeSpy = vi.fn();
    const saveSpy = vi.fn();

    const { getByText, getByRole } = render(SettingsDrawer, {
      open: true,
      onclose: closeSpy,
      onsave: saveSpy,
    });

    expect(getActiveLocale()).toBe("en");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");

    // Change language to Spanish
    const esOption = getByText("Español");
    await fireEvent.click(esOption);
    expect(getActiveLocale()).toBe("es");

    // Labels re-render live in the previewed language
    const darkOption = getByText("Oscuro");
    await fireEvent.click(darkOption);
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");

    // Close without saving
    const closeBtn = getByRole("button", { name: "Cerrar" });
    await fireEvent.click(closeBtn);

    expect(closeSpy).toHaveBeenCalled();
    // Verify reverted to initial values
    expect(getActiveLocale()).toBe("en");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    expect(saveSpy).not.toHaveBeenCalled();
  });

  it("commits theme and language changes when user clicks save", async () => {
    const closeSpy = vi.fn();
    const saveSpy = vi.fn();

    const { getByText, getByRole } = render(SettingsDrawer, {
      open: true,
      onclose: closeSpy,
      onsave: saveSpy,
    });

    // Change language to Spanish
    const esOption = getByText("Español");
    await fireEvent.click(esOption);

    // Click Save
    const saveBtn = getByRole("button", { name: "Guardar Ajustes" });
    await fireEvent.click(saveBtn);

    expect(saveSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        language: "es",
      }),
      "Initial User",
      "user@example.com",
      undefined
    );
  });
});
