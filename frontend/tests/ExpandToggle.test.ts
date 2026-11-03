import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render } from "@testing-library/svelte";
import ExpandToggle from "../src/lib/components/ExpandToggle.svelte";

describe("ExpandToggle.svelte", () => {
  afterEach(() => cleanup());

  it("is a keyboard-reachable button that reports its state and what it controls", async () => {
    const onclick = vi.fn();
    const { getByRole } = render(ExpandToggle, {
      expanded: false,
      label: "Show details",
      count: 3,
      controls: "panel-1",
      onclick,
    });
    const button = getByRole("button", { name: "Show details" });
    expect(button.getAttribute("aria-expanded")).toBe("false");
    expect(button.getAttribute("aria-controls")).toBe("panel-1");
    expect(button.getAttribute("title")).toBe("Show details");
    expect(button.textContent?.trim()).toBe("3");
    button.focus();
    expect(document.activeElement).toBe(button);
    await fireEvent.click(button);
    expect(onclick).toHaveBeenCalledOnce();
  });

  it("reflects the expanded state and omits the count when unknown", () => {
    const { getByRole, container } = render(ExpandToggle, {
      expanded: true,
      label: "Hide details",
      title: "2 files",
      onclick: vi.fn(),
    });
    const button = getByRole("button", { name: "Hide details" });
    expect(button.getAttribute("aria-expanded")).toBe("true");
    expect(button.getAttribute("title")).toBe("2 files");
    expect(button.classList).toContain("expanded");
    expect(container.querySelector(".expand-count")).toBeNull();
  });

  it("flags hidden content that needs attention, but not healthy or idle content", () => {
    const attention = render(ExpandToggle, {
      expanded: false,
      label: "Show",
      attention: "conflict",
      onclick: vi.fn(),
    });
    expect(attention.container.querySelector(".expand-attention")).not.toBeNull();
    cleanup();
    for (const tone of ["ok", "idle"] as const) {
      const quiet = render(ExpandToggle, {
        expanded: false,
        label: "Show",
        attention: tone,
        onclick: vi.fn(),
      });
      expect(quiet.container.querySelector(".expand-attention")).toBeNull();
      cleanup();
    }
  });
});
