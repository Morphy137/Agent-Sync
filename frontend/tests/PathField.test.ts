import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import PathField from "../src/lib/components/PathField.svelte";
import * as App from "../wailsjs/go/main/App";
import { setLocale } from "../src/lib/i18n";

vi.mock("../wailsjs/go/main/App", () => ({
  InspectPath: vi.fn(),
  SelectFolder: vi.fn(),
  CreateFolder: vi.fn(),
}));

describe("PathField.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
  });

  afterEach(() => {
    cleanup();
  });

  it("shows empty path hint when value is empty", async () => {
    const { getByText } = render(PathField, {
      value: "",
      mode: "root",
    });

    expect(getByText("Please enter a folder path")).toBeDefined();
  });

  it("shows valid folder status when path exists as dir and is writable", async () => {
    vi.mocked(App.InspectPath).mockResolvedValue({
      normalized: "C:/valid/dir",
      exists: true,
      isDir: true,
      writable: true,
      isInsideRoot: false,
    } as any);

    const { getByText } = render(PathField, {
      value: "C:/valid/dir",
      mode: "root",
    });

    await waitFor(() => {
      expect(getByText("Valid folder")).toBeDefined();
    });
  });

  it("shows non-existent message and create button when allowCreate is true", async () => {
    vi.mocked(App.InspectPath).mockResolvedValue({
      normalized: "C:/missing/dir",
      exists: false,
      isDir: false,
      writable: false,
      isInsideRoot: false,
    } as any);

    const { getByText } = render(PathField, {
      value: "C:/missing/dir",
      mode: "root",
      allowCreate: true,
    });

    await waitFor(() => {
      expect(getByText("Folder does not exist")).toBeDefined();
      expect(getByText("[Create folder]")).toBeDefined();
    });
  });

  it("shows error when a backup path is inside repository root", async () => {
    vi.mocked(App.InspectPath).mockResolvedValue({
      normalized: "C:/repo/root/sub",
      exists: true,
      isDir: true,
      writable: true,
      isInsideRoot: true,
    } as any);

    const { getByText } = render(PathField, {
      value: "C:/repo/root/sub",
      mode: "backup",
    });

    await waitFor(() => {
      expect(getByText("Target cannot be inside repository root")).toBeDefined();
    });
  });

  it("fills the input with the folder chosen in the browse dialog", async () => {
    vi.mocked(App.SelectFolder).mockResolvedValue("C:/picked/dir");
    vi.mocked(App.InspectPath).mockResolvedValue({
      exists: true,
      isDir: true,
      writable: true,
    } as any);

    const { getByRole, getByDisplayValue } = render(PathField, { value: "", mode: "root" });
    await fireEvent.click(getByRole("button", { name: "Browse" }));

    await waitFor(() => {
      expect(getByDisplayValue("C:/picked/dir")).toBeDefined();
    });
  });

  it("shows an error when the browse dialog fails", async () => {
    vi.mocked(App.SelectFolder).mockRejectedValue("dialog exploded");

    const { getByRole, getByText } = render(PathField, { value: "", mode: "root" });
    await fireEvent.click(getByRole("button", { name: "Browse" }));

    await waitFor(() => {
      expect(getByText("Could not open the folder picker: dialog exploded")).toBeDefined();
    });
  });
});
