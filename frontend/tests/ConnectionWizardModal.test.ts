import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import ConnectionWizardModal from "../src/lib/components/ConnectionWizardModal.svelte";
import * as App from "../wailsjs/go/main/App";
import { appState } from "../src/lib/stores/appState.svelte";
import { setLocale } from "../src/lib/i18n";

vi.mock("../wailsjs/go/main/App", () => ({
  ListProfiles: vi.fn(),
  DetectAgentFolders: vi.fn(),
  ListTargetEntries: vi.fn(),
  BeginConnection: vi.fn(),
  CommitConnection: vi.fn(),
  CancelConnection: vi.fn(),
  InspectPath: vi.fn(),
  SelectFolder: vi.fn(),
  CreateFolder: vi.fn(),
}));

describe("ConnectionWizardModal.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    appState.config = {
      rootPath: "C:/repo/root",
      connections: [],
    } as any;

    vi.mocked(App.ListProfiles).mockResolvedValue([
      { name: "CLAUDE", path: "C:/repo/root/CLAUDE", fileCount: 3 },
      { name: "GEMINI", path: "C:/repo/root/GEMINI", fileCount: 1 },
    ] as any);

    vi.mocked(App.DetectAgentFolders).mockResolvedValue([
      {
        path: "C:/Users/User/.claude",
        label: "Claude Code",
        suggestedProfile: "CLAUDE",
      },
    ] as any);
  });

  afterEach(() => {
    cleanup();
  });

  it("renders profiles and allows selecting an existing profile in step 1", async () => {
    const { getByText } = render(ConnectionWizardModal, {
      open: true,
      onclose: vi.fn(),
    });

    await waitFor(() => {
      expect(getByText("CLAUDE")).toBeDefined();
      expect(getByText("GEMINI")).toBeDefined();
      expect(getByText("Create new folder in repository")).toBeDefined();
    });
  });

  it("validates new folder names correctly", async () => {
    const { getByText, getByPlaceholderText, getAllByRole } = render(ConnectionWizardModal, {
      open: true,
      onclose: vi.fn(),
    });

    await waitFor(() => {
      expect(getAllByRole("radio")).toHaveLength(3);
    });

    const radios = getAllByRole("radio");
    await fireEvent.change(radios[2]);

    const input = getByPlaceholderText("e.g. CLAUDE");
    expect(input).toBeDefined();

    // Test invalid characters: "invalid*name"
    await fireEvent.input(input, { target: { value: "invalid*name" } });
    await waitFor(() => {
      expect(getByText("The folder name contains invalid characters or is a reserved name.")).toBeDefined();
    });

    // Test reserved name: "CON"
    await fireEvent.input(input, { target: { value: "CON" } });
    await waitFor(() => {
      expect(getByText("The folder name contains invalid characters or is a reserved name.")).toBeDefined();
    });

    // Test already exists: "CLAUDE"
    await fireEvent.input(input, { target: { value: "CLAUDE" } });
    await waitFor(() => {
      expect(getByText("The folder already exists.")).toBeDefined();
    });

    // Test valid name: "NEW_AGENT"
    await fireEvent.input(input, { target: { value: "NEW_AGENT" } });
    await waitFor(() => {
      const nextBtn = getByText("Next").closest("button") as HTMLButtonElement;
      expect(nextBtn.disabled).toBe(false);
    });
  });

  it("navigates through steps 1 to 3", async () => {
    vi.mocked(App.InspectPath).mockResolvedValue({
      normalized: "C:/Users/User/.claude",
      exists: true,
      isDir: true,
      writable: true,
      isInsideRoot: false,
      alreadyConnected: false,
    } as any);

    vi.mocked(App.BeginConnection).mockResolvedValue({
      id: "draft-123",
      profile: "CLAUDE",
      targetPath: "C:/Users/User/.claude",
      createProfile: false,
      importPaths: [],
      preview: {
        toImport: [],
        toCreateInTarget: ["CLAUDE.md"],
        toOverwrite: [],
        differences: [
          {
            relPath: "settings.json",
            repoHash: "h1",
            targetHash: "h2",
          },
        ],
      },
    } as any);

    const { getByText, getAllByRole } = render(ConnectionWizardModal, {
      open: true,
      onclose: vi.fn(),
    });

    await waitFor(() => {
      expect(getAllByRole("radio")).toHaveLength(3);
    });

    // Select CLAUDE radio
    const radios = getAllByRole("radio");
    await fireEvent.change(radios[0]);

    // Advance to Step 2
    const nextBtn = getByText("Next");
    await fireEvent.click(nextBtn);

    // Verify Step 2 reached
    await waitFor(() => {
      expect(getByText("Local target folder")).toBeDefined();
    });

    // Select suggestion chip
    const chip = getByText("Claude Code");
    await fireEvent.click(chip);

    // Advance to Step 3
    let step2NextBtn: HTMLElement | null = null;
    await waitFor(() => {
      const el = getByText("Next");
      expect((el.closest("button") as HTMLButtonElement).disabled).toBe(false);
      step2NextBtn = el;
    });
    await fireEvent.click(step2NextBtn!);

    // Verify Step 3 reached
    await waitFor(() => {
      expect(App.BeginConnection).toHaveBeenCalled();
      expect(getByText("Review planned changes before establishing the connection.")).toBeDefined();
      expect(getByText("settings.json")).toBeDefined();
      expect(getByText("Connect")).toBeDefined();
    });
  });

  it("cancels draft if modal is closed during step 3", async () => {
    vi.mocked(App.InspectPath).mockResolvedValue({
      normalized: "C:/Users/User/.claude",
      exists: true,
      isDir: true,
      writable: true,
      isInsideRoot: false,
      alreadyConnected: false,
    } as any);

    vi.mocked(App.BeginConnection).mockResolvedValue({
      id: "draft-456",
      profile: "CLAUDE",
      targetPath: "C:/Users/User/.claude",
      createProfile: false,
      importPaths: [],
      preview: {
        toImport: [],
        toCreateInTarget: [],
        toOverwrite: [],
        differences: [],
      },
    } as any);

    const closeSpy = vi.fn();
    const { getByText, getByRole, getAllByRole } = render(ConnectionWizardModal, {
      open: true,
      onclose: closeSpy,
    });

    await waitFor(() => {
      expect(getAllByRole("radio")).toHaveLength(3);
    });
    const radios = getAllByRole("radio");
    await fireEvent.change(radios[0]);

    const nextBtn = getByText("Next");
    await fireEvent.click(nextBtn);

    await waitFor(() => {
      expect(getByText("Local target folder")).toBeDefined();
    });

    const chip = getByText("Claude Code");
    await fireEvent.click(chip);

    let step2NextBtn2: HTMLElement | null = null;
    await waitFor(() => {
      const el = getByText("Next");
      expect((el.closest("button") as HTMLButtonElement).disabled).toBe(false);
      step2NextBtn2 = el;
    });
    await fireEvent.click(step2NextBtn2!);

    await waitFor(() => {
      expect(getByText("Connect")).toBeDefined();
    });

    // Close modal via close button
    const closeBtn = getByRole("button", { name: "Close" });
    await fireEvent.click(closeBtn);

    expect(App.CancelConnection).toHaveBeenCalledWith("draft-456");
    expect(closeSpy).toHaveBeenCalled();
  });
});
