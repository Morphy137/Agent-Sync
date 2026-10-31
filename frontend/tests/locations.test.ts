import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";

const mockApi = vi.hoisted(() => ({
  listLocations: vi.fn(),
  saveLocation: vi.fn(),
  removeLocation: vi.fn(),
  openPath: vi.fn(),
  getRecognizersPath: vi.fn(),
}));

vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));

import LocationsView from "../src/lib/views/LocationsView.svelte";

const sampleLocations = [
  {
    alias: "home",
    path: "C:/home",
    builtin: true,
    exists: true,
    profileId: "",
    profileLabel: "",
    ruleCount: 0,
  },
  {
    alias: "claude",
    path: "C:/home/.claude",
    builtin: false,
    exists: true,
    profileId: "claude-code",
    profileLabel: "Claude Code",
    ruleCount: 2,
  },
  {
    alias: "obsidian",
    path: "",
    builtin: false,
    exists: false,
    profileId: "",
    profileLabel: "",
    ruleCount: 1,
  },
];

describe("LocationsView.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    mockApi.listLocations.mockResolvedValue(sampleLocations);
  });

  afterEach(() => cleanup());

  it("lists mapped locations with detected tools and highlights unmapped ones", async () => {
    const { findByText, getByText } = render(LocationsView);
    expect(await findByText("Detected: Claude Code")).toBeDefined();
    expect(getByText("Locations used by rules but not set on this PC")).toBeDefined();
    expect(getByText("obsidian")).toBeDefined();
    expect(getByText("Built-in")).toBeDefined();
  });

  it("removes a location after confirmation and warns when rules use it", async () => {
    mockApi.removeLocation.mockResolvedValue({});
    const { findAllByRole, findByText, getAllByText } = render(LocationsView);

    const removeButtons = await findAllByRole("button", { name: "Remove" });
    await fireEvent.click(removeButtons[0]);
    expect(await findByText(/2 destination\(s\) use it/)).toBeDefined();

    const confirm = getAllByText("Remove").at(-1)!;
    await fireEvent.click(confirm);
    await waitFor(() => expect(mockApi.removeLocation).toHaveBeenCalledWith("claude"));
  });

  it("opens the recognizers file", async () => {
    mockApi.getRecognizersPath.mockResolvedValue("C:/config/recognizers.json");
    mockApi.openPath.mockResolvedValue(undefined);
    const { findByText } = render(LocationsView);
    await fireEvent.click(await findByText("Open recognizers file"));
    await waitFor(() =>
      expect(mockApi.openPath).toHaveBeenCalledWith("C:/config/recognizers.json")
    );
  });
});
