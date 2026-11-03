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
    expect(getByText("Locations not configured on this computer")).toBeDefined();
    expect(getByText("Used by 1 destination")).toBeDefined();
    expect(getByText("Used by 2 destinations")).toBeDefined();
    expect(getByText("obsidian")).toBeDefined();
    expect(getByText("Built-in")).toBeDefined();
  });

  it("lays every location out in the same zones, keeping empty ones reserved", async () => {
    const { findByText, container } = render(LocationsView);
    await findByText("Detected: Claude Code");
    const rows = [...container.querySelectorAll("ul.card > li.row")];
    expect(rows).toHaveLength(2);
    for (const row of rows) {
      expect([...row.children].map((child) => child.className.split(" ")[0])).toEqual([
        "alias",
        "path",
        "tags",
        "hint",
        "row-actions",
      ]);
      // The built-in location has no edit button but keeps its slot, so icons do not shift.
      expect(row.querySelector(".row-actions")!.children).toHaveLength(3);
    }
    expect(rows[0].querySelector(".action-slot")).not.toBeNull();
    expect(rows[1].querySelector(".action-slot")).toBeNull();
  });

  it("keeps the full path available for tooltip and screen readers when it is cut", async () => {
    const { findByText, container } = render(LocationsView);
    await findByText("Detected: Claude Code");
    const path = container.querySelector("ul.card > li.row .middle-ellipsis")!;
    expect(path.getAttribute("title")).toBe("C:/home");
    const claude = container.querySelectorAll("ul.card > li.row .middle-ellipsis")[1];
    expect(claude.getAttribute("title")).toBe("C:/home/.claude");
    expect(claude.textContent).toBe("C:/home/.claude");
    expect(claude.querySelector(".end")!.textContent).toBe("/.claude");
  });

  it("explains that folders are named per computer", async () => {
    const { findByText } = render(LocationsView);
    expect(
      await findByText(
        "Name the folders on this PC. Rules use the name, so each computer can have its own path."
      )
    ).toBeDefined();
  });

  it("shows singular and plural destination counts in Spanish", async () => {
    setLocale("es");
    const { findByText, getByText } = render(LocationsView);
    expect(await findByText("Usada por 1 destino")).toBeDefined();
    expect(getByText("Usada por 2 destinos")).toBeDefined();
  });

  it("removes a location after confirmation and warns when rules use it", async () => {
    mockApi.removeLocation.mockResolvedValue({});
    const { findAllByRole, findByText, getAllByText } = render(LocationsView);

    const removeButtons = await findAllByRole("button", { name: "Remove" });
    await fireEvent.click(removeButtons[1]);
    expect(await findByText(/2 destinations use it/)).toBeDefined();

    const confirm = getAllByText("Remove").at(-1)!;
    await fireEvent.click(confirm);
    await waitFor(() => expect(mockApi.removeLocation).toHaveBeenCalledWith("claude"));
  });

  it("allows removing the built-in home location", async () => {
    mockApi.removeLocation.mockResolvedValue({});
    const { findAllByRole, getAllByText } = render(LocationsView);
    await fireEvent.click((await findAllByRole("button", { name: "Remove" }))[0]);
    await fireEvent.click(getAllByText("Remove").at(-1)!);
    await waitFor(() => expect(mockApi.removeLocation).toHaveBeenCalledWith("home"));
  });

  it("opens the recognizers file", async () => {
    mockApi.getRecognizersPath.mockResolvedValue("C:/config/recognizers.json");
    mockApi.openPath.mockResolvedValue(undefined);
    const { findByText } = render(LocationsView);
    await fireEvent.click(await findByText("Open detection settings"));
    await waitFor(() =>
      expect(mockApi.openPath).toHaveBeenCalledWith("C:/config/recognizers.json")
    );
  });
});
