import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render } from "@testing-library/svelte";
import ViewTabs from "../src/lib/components/ViewTabs.svelte";
import SegmentedControl from "../src/lib/components/SegmentedControl.svelte";
import SyncPreviewModal from "../src/lib/components/SyncPreviewModal.svelte";
import { isValidRemoteUrl, repoNameFromUrl } from "../src/lib/components/remote/remoteUrl";
import { setLocale } from "../src/lib/i18n";

afterEach(() => cleanup());

describe("keyboard navigation", () => {
  it("moves focus between tabs with repeated arrow keys", async () => {
    setLocale("en");
    const onchange = vi.fn();
    const { getByRole } = render(ViewTabs, { value: "rules", onchange });
    const first = getByRole("tab", { name: "Sync rules" });
    first.focus();
    await fireEvent.keyDown(first, { key: "ArrowRight" });
    expect(document.activeElement).toBe(getByRole("tab", { name: "Folder connections" }));
    await fireEvent.keyDown(document.activeElement!, { key: "ArrowRight" });
    expect(onchange).toHaveBeenLastCalledWith("subscriptions");
    expect(document.activeElement).toBe(getByRole("tab", { name: "Subscriptions" }));
  });

  it("moves focus and selects an adjacent segment", async () => {
    const onchange = vi.fn();
    const { getByRole } = render(SegmentedControl, {
      label: "Choice",
      options: [
        { value: "a", label: "First" },
        { value: "b", label: "Second" },
      ],
      value: "a",
      onchange,
    });
    const first = getByRole("radio", { name: "First" });
    const second = getByRole("radio", { name: "Second" });
    first.focus();
    await fireEvent.keyDown(first, { key: "ArrowRight" });
    expect(onchange).toHaveBeenCalledWith("b");
    expect(document.activeElement).toBe(second);
  });
});

describe("local repository paths", () => {
  it("accepts absolute Windows paths with spaces and derives the folder name", () => {
    const path = String.raw`C:\My Repos\example.git`;
    expect(isValidRemoteUrl(path)).toBe(true);
    expect(repoNameFromUrl(path)).toBe("example");
    expect(isValidRemoteUrl(String.raw`C:\repos\ext::bad`)).toBe(false);
    expect(isValidRemoteUrl("-danger")).toBe(false);
    expect(isValidRemoteUrl(String.raw`\\server`)).toBe(false);
  });
});

describe("sync preview", () => {
  it("prevents another confirmation while syncing", async () => {
    setLocale("en");
    const onconfirm = vi.fn();
    const { getByRole } = render(SyncPreviewModal, {
      open: true,
      plan: null,
      syncing: true,
      onclose: vi.fn(),
      onconfirm,
    });
    const button = getByRole("button", { name: /sync/i }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    await fireEvent.click(button);
    expect(onconfirm).not.toHaveBeenCalled();
  });
});
