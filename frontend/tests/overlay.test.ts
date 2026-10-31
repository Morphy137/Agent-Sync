import { afterEach, describe, expect, it, vi } from "vitest";
import { overlay } from "../src/lib/components/overlay";

afterEach(() => {
  document.body.innerHTML = "";
});

describe("overlay", () => {
  it("closes only the top overlay and restores focus as overlays close", () => {
    const trigger = document.createElement("button");
    const outer = document.createElement("div");
    const inner = document.createElement("div");
    outer.tabIndex = inner.tabIndex = -1;
    document.body.append(trigger, outer, inner);
    trigger.focus();
    const closeOuter = vi.fn();
    const closeInner = vi.fn();
    const outerAction = overlay(outer, closeOuter);
    const innerAction = overlay(inner, closeInner);
    expect(document.activeElement).toBe(inner);
    document.activeElement?.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Escape", bubbles: true })
    );
    expect(closeInner).toHaveBeenCalledOnce();
    expect(closeOuter).not.toHaveBeenCalled();
    innerAction.destroy();
    expect(document.activeElement).toBe(outer);
    outerAction.destroy();
    expect(document.activeElement).toBe(trigger);
  });

  it("contains focus and wraps Tab within the top overlay", () => {
    const outside = document.createElement("button");
    const dialog = document.createElement("div");
    dialog.tabIndex = -1;
    const first = document.createElement("button");
    const last = document.createElement("button");
    const skipped = document.createElement("button");
    skipped.tabIndex = -1;
    dialog.append(first, last, skipped);
    document.body.append(outside, dialog);
    const action = overlay(dialog, vi.fn());
    dialog.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Tab", shiftKey: true, bubbles: true, cancelable: true })
    );
    expect(document.activeElement).toBe(last);
    last.focus();
    last.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true })
    );
    expect(document.activeElement).toBe(first);
    first.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Tab", shiftKey: true, bubbles: true, cancelable: true })
    );
    expect(document.activeElement).toBe(last);
    outside.focus();
    expect(document.activeElement).toBe(dialog);
    action.destroy();
  });
});
