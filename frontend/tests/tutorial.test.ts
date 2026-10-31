import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";
import { appState } from "../src/lib/stores/appState.svelte";
import TutorialModal from "../src/lib/components/TutorialModal.svelte";

const TOTAL_STEPS = 6;

describe("TutorialModal.svelte", () => {
  beforeEach(() => {
    setLocale("en");
    appState.activeView = "connections";
  });

  afterEach(() => cleanup());

  it("walks through the steps, switching the tab behind it, and finishes", async () => {
    const onfinish = vi.fn();
    const { getByRole, getByText } = render(TutorialModal, { open: true, onfinish });

    expect(getByText("Welcome to AgentSync")).toBeDefined();
    for (let step = 1; step < TOTAL_STEPS; step++) {
      await fireEvent.click(getByRole("button", { name: "Next" }));
    }
    expect(getByText("All set")).toBeDefined();
    expect(appState.activeView).toBe("rules");

    await fireEvent.click(getByRole("button", { name: "Get started" }));
    expect(onfinish).toHaveBeenCalledOnce();
  });

  it("can be skipped and navigated back", async () => {
    const onfinish = vi.fn();
    const { getByRole, getByText } = render(TutorialModal, { open: true, onfinish });

    await fireEvent.click(getByRole("button", { name: "Next" }));
    expect(getByText("Rules")).toBeDefined();
    await fireEvent.click(getByRole("button", { name: "Back" }));
    expect(getByText("Welcome to AgentSync")).toBeDefined();

    await fireEvent.click(getByRole("button", { name: "Skip" }));
    expect(onfinish).toHaveBeenCalledOnce();
  });
});
