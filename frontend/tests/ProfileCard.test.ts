import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, cleanup } from "@testing-library/svelte";
import { tick } from "svelte";
import ProfileCard from "../src/lib/components/ProfileCard.svelte";
import { setLocale } from "../src/lib/i18n";

function renderConnectedCard() {
  return render(ProfileCard, {
    profile: { name: "CLAUDE", path: "C:/repo/CLAUDE", fileCount: 3 } as any,
    connection: { id: "c1", profile: "CLAUDE", targetPath: "C:/Users/me/.claude" } as any,
    status: { status: "in_sync", files: [] } as any,
    onconnect: vi.fn(),
    onupdate: vi.fn(),
    onbackups: vi.fn(),
    onchangeTarget: vi.fn(),
    onopenExplorer: vi.fn(),
    ondisconnect: vi.fn(),
  });
}

describe("ProfileCard.svelte", () => {
  beforeEach(() => {
    setLocale("en");
  });

  afterEach(() => {
    cleanup();
  });

  it("renders translated menu items instead of raw keys", async () => {
    const { getByLabelText, getByText, queryByText } = renderConnectedCard();
    await fireEvent.click(getByLabelText("More options"));

    expect(getByText("Open repository folder")).toBeDefined();
    expect(getByText("Open target folder")).toBeDefined();
    expect(getByText("Change target folder")).toBeDefined();
    expect(getByText("Disconnect")).toBeDefined();
    expect(queryByText(/connections\./)).toBeNull();
  });

  it("closes the menu when clicking outside", async () => {
    const { getByLabelText, queryByRole } = renderConnectedCard();
    await fireEvent.click(getByLabelText("More options"));
    expect(queryByRole("menu")).not.toBeNull();

    await fireEvent.pointerDown(document.body);
    expect(queryByRole("menu")).toBeNull();
  });

  it("keeps the menu open when clicking inside it", async () => {
    const { getByLabelText, getByRole } = renderConnectedCard();
    await fireEvent.click(getByLabelText("More options"));

    await fireEvent.pointerDown(getByRole("menu"));
    expect(getByRole("menu")).toBeDefined();
  });

  it("closes the menu on Escape", async () => {
    const { getByLabelText, queryByRole } = renderConnectedCard();
    await fireEvent.click(getByLabelText("More options"));

    await fireEvent.keyDown(window, { key: "Escape" });
    expect(queryByRole("menu")).toBeNull();
  });

  it("re-renders text when the locale changes after mount", async () => {
    const { getByText } = renderConnectedCard();
    expect(getByText("In sync")).toBeDefined();

    setLocale("es");
    await tick();
    expect(getByText("Sincronizado")).toBeDefined();
    expect(getByText("Repositorio")).toBeDefined();
  });
});
