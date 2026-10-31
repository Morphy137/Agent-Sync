import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render } from "@testing-library/svelte";
import BackupsDrawer from "../src/lib/components/BackupsDrawer.svelte";
import { setLocale } from "../src/lib/i18n";

afterEach(cleanup);

describe("BackupsDrawer", () => {
  it("clears restore confirmation when closed and reopened", async () => {
    setLocale("en");
    const props = {
      open: true,
      profileName: "Profile",
      backups: [
        { id: "1", createdAt: new Date().toISOString(), reason: "sync", files: [], path: "backup" },
      ] as any,
      onclose: vi.fn(),
      onrestore: vi.fn(),
      onopenFolder: vi.fn(),
    };
    const view = render(BackupsDrawer, props);
    await fireEvent.click(view.getByRole("button", { name: /restore/i }));
    expect(view.getByRole("button", { name: /confirm/i })).toBeTruthy();
    await view.rerender({ open: false });
    await view.rerender({ open: true });
    expect(view.queryByRole("button", { name: /confirm/i })).toBeNull();
    expect(props.onrestore).not.toHaveBeenCalled();
  });
});
