import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, waitFor } from "@testing-library/svelte";
import RuleCard from "../src/lib/components/rules/RuleCard.svelte";
import { appState } from "../src/lib/stores/appState.svelte";

describe("subscription rule source", () => {
  afterEach(() => {
    cleanup();
    appState.config = null;
  });

  it("shows the subscription name and falls back to its ID when unavailable", async () => {
    appState.config = { subscriptions: [{ id: "s_123", name: "Shared skills" }] } as any;
    const { getByText } = render(RuleCard, {
      status: {
        rule: {
          id: "r_1",
          name: "My rule",
          enabled: true,
          source: { origin: "subscription", subscription: "s_123", path: "AGENTS.md" },
          kind: "auto",
          destinations: [],
        },
        scope: "local",
        destinations: [],
      } as any,
      sourcePath: "",
      ontoggle: vi.fn(),
      onedit: vi.fn(),
      onduplicate: vi.fn(),
      ondelete: vi.fn(),
      onopen: vi.fn(),
      onreview: vi.fn(),
    });

    expect(getByText("Shared skills").getAttribute("title")).toBe("s_123");
    expect(getByText("Subscription").getAttribute("title")).toContain("rule saved");
    appState.config = { subscriptions: [] } as any;
    await waitFor(() => expect(getByText("s_123")).toBeDefined());
  });
});
