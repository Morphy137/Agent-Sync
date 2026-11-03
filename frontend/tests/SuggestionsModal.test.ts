import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";
import { appState } from "../src/lib/stores/appState.svelte";

const mockApi = vi.hoisted(() => ({
  listSubscriptions: vi.fn(),
  addSubscription: vi.fn(),
  listSubscriptionSuggestions: vi.fn(),
  applySubscriptionSuggestions: vi.fn(),
  getSubscriptionTree: vi.fn(),
  selectFolder: vi.fn(),
  openURL: vi.fn(),
}));

vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));

import SuggestionsModal from "../src/lib/components/subscriptions/SuggestionsModal.svelte";
import SubscriptionCard from "../src/lib/components/subscriptions/SubscriptionCard.svelte";
import SubscriptionsView from "../src/lib/views/SubscriptionsView.svelte";
import {
  detectedFolders,
  failureText,
  initialSelection,
  mappingsFor,
  missingFolderAliases,
} from "../src/lib/components/subscriptions/suggestions";

const mappedDest = {
  location: "claude",
  path: "CLAUDE.md",
  mode: "one_way",
  mapped: true,
  targetPath: "/home/me/.claude/CLAUDE.md",
  targetExists: true,
  detectedPath: "",
  applied: false,
};

const unmappedDest = {
  location: "codex",
  path: "skills/review",
  mode: "create_only",
  mapped: false,
  targetPath: "",
  targetExists: false,
  detectedPath: "",
  applied: false,
};

function suggestion(overrides: Record<string, unknown> = {}) {
  return {
    id: "a1",
    name: "Claude instructions",
    description: "Main instructions",
    source: "AGENTS/Claude/CLAUDE-AGENTS.md",
    include: [],
    kind: "auto",
    sourceExists: true,
    applied: false,
    valid: true,
    invalidReason: "",
    destinations: [mappedDest],
    ...overrides,
  };
}

const claude = suggestion();
const skill = suggestion({
  id: "a2",
  name: "Review skill",
  description: "",
  source: "skills/review",
  kind: "skill",
  destinations: [unmappedDest],
});
const applied = suggestion({
  id: "a3",
  name: "Gemini instructions",
  source: "AGENTS/Gemini/GEMINI-AGENTS.md",
  applied: true,
  destinations: [
    {
      ...mappedDest,
      location: "gemini",
      path: "GEMINI.md",
      targetPath: "/home/me/.gemini/GEMINI.md",
      applied: true,
    },
  ],
});
const invalid = suggestion({
  id: "a4",
  name: "Sneaky",
  source: "../x.md",
  valid: false,
  invalidReason: "invalid_source_path",
  destinations: [{ ...mappedDest, path: "../../.bashrc", mapped: false, targetPath: "" }],
});

const subscription = { id: "s_1", name: "Morphy-Sync" };

function renderModal(onclose = vi.fn()) {
  const view = render(SuggestionsModal, { open: true, subscription, onclose });
  return { ...view, onclose };
}

describe("suggestions helpers", () => {
  it("selects only valid, not yet applied suggestions", () => {
    expect(initialSelection([claude, skill, applied, invalid] as never)).toEqual(["a1", "a2"]);
  });

  it("offers recognized folders and requires one for unmapped locations", () => {
    const detected = suggestion({
      id: "d",
      destinations: [{ ...unmappedDest, detectedPath: "/home/me/.codex" }],
    });
    expect(detectedFolders([claude, detected] as never)).toEqual({ codex: "/home/me/.codex" });
    const selected = new Set(["a2"]);
    expect(missingFolderAliases([skill] as never, selected, {})).toEqual(["codex"]);
    expect(missingFolderAliases([skill] as never, selected, { codex: "/x" })).toEqual([]);
  });

  it("sends only the folders of the selected suggestions", () => {
    const folders = { codex: "/x", other: "/y" };
    expect(mappingsFor([claude, skill] as never, new Set(["a1"]), folders)).toEqual({});
    expect(mappingsFor([claude, skill] as never, new Set(["a2"]), folders)).toEqual({
      codex: "/x",
    });
  });

  it("explains failures in plain words", () => {
    setLocale("en");
    const base = { id: "a", status: "failed", ruleId: "", errorReason: "", collisions: 0 };
    expect(
      failureText({ ...base, errorCode: "location_unmapped", errorAlias: "codex" } as never)
    ).toBe("Choose a folder for codex.");
    expect(
      failureText({ ...base, errorCode: "target_not_found", errorAlias: "codex" } as never)
    ).toBe("That folder cannot be used for codex.");
    expect(
      failureText({
        ...base,
        errorCode: "invalid_suggestions",
        errorReason: "unknown_suggestion",
        errorAlias: "",
      } as never)
    ).toBe("This suggestion is no longer in the repository.");
  });
});

describe("SuggestionsModal.svelte", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    mockApi.listSubscriptionSuggestions.mockResolvedValue([claude, skill, applied, invalid]);
    mockApi.selectFolder.mockResolvedValue("/home/me/.codex");
  });

  afterEach(() => cleanup());

  it("lists each suggestion with where its files will land", async () => {
    const { findByText, getByText, getAllByRole } = renderModal();
    expect(await findByText("Claude instructions")).toBeDefined();
    expect(getByText("AGENTS/Claude/CLAUDE-AGENTS.md")).toBeDefined();
    expect(getByText("/home/me/.claude/CLAUDE.md")).toBeDefined();
    expect(getByText("A file is already there and will be replaced.")).toBeDefined();
    expect(getByText("The source path is not allowed.")).toBeDefined();
    expect(getByText("../../.bashrc")).toBeDefined();

    const checkboxes = getAllByRole("checkbox") as HTMLInputElement[];
    expect(checkboxes.map((box) => box.checked)).toEqual([true, true, false, false]);
    expect(checkboxes.map((box) => box.disabled)).toEqual([false, false, true, true]);
  });

  it("asks for a folder for an unmapped location before applying", async () => {
    const { findByRole, getByRole, getByText } = renderModal();
    const apply = (await findByRole("button", { name: "Apply 2 rules" })) as HTMLButtonElement;
    expect(apply.disabled).toBe(true);
    expect(getByText("codex is not set on this PC. Choose its folder.")).toBeDefined();

    await fireEvent.click(getByRole("button", { name: "Choose folder" }));
    await waitFor(() =>
      expect(mockApi.selectFolder).toHaveBeenCalledWith("Choose the folder for codex")
    );
    await waitFor(() => expect(apply.disabled).toBe(false));
    expect(getByText("/home/me/.codex")).toBeDefined();
  });

  it("unticking the rule that needs a folder lets the others apply", async () => {
    const { findByRole, getAllByRole } = renderModal();
    const apply = (await findByRole("button", { name: "Apply 2 rules" })) as HTMLButtonElement;
    await fireEvent.click(getAllByRole("checkbox")[1]);
    expect(apply.textContent?.trim()).toBe("Apply 1 rule");
    expect(apply.disabled).toBe(false);
  });

  it("prefills a recognized folder but still shows it for confirmation", async () => {
    mockApi.listSubscriptionSuggestions.mockResolvedValue([
      suggestion({
        id: "a2",
        name: "Review skill",
        destinations: [{ ...unmappedDest, detectedPath: "/home/me/.codex" }],
      }),
    ]);
    const { findByRole, getByText } = renderModal();
    const apply = (await findByRole("button", { name: "Apply 1 rule" })) as HTMLButtonElement;
    expect(getByText("codex found on this PC. Confirm or change the folder.")).toBeDefined();
    expect(getByText("/home/me/.codex")).toBeDefined();
    expect(apply.disabled).toBe(false);
  });

  it("applies the selection with the chosen folders, then reports and closes", async () => {
    mockApi.applySubscriptionSuggestions.mockResolvedValue({
      applied: 2,
      results: [
        { id: "a1", status: "applied", ruleId: "r1", collisions: 0 },
        { id: "a2", status: "applied", ruleId: "r2", collisions: 0 },
      ],
    });
    const addToast = vi.spyOn(appState, "addToast");
    const afterRuleChange = vi.spyOn(appState, "afterRuleChange").mockImplementation(() => {});
    const { findByRole, getByRole, onclose } = renderModal();

    await findByRole("button", { name: "Apply 2 rules" });
    await fireEvent.click(getByRole("button", { name: "Choose folder" }));
    await waitFor(() =>
      expect((getByRole("button", { name: "Apply 2 rules" }) as HTMLButtonElement).disabled).toBe(
        false
      )
    );
    await fireEvent.click(getByRole("button", { name: "Apply 2 rules" }));

    await waitFor(() =>
      expect(mockApi.applySubscriptionSuggestions).toHaveBeenCalledWith("s_1", ["a1", "a2"], {
        codex: "/home/me/.codex",
      })
    );
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(addToast).toHaveBeenCalledWith("success", "2 rules created. Sync to copy the files.");
    expect(afterRuleChange).toHaveBeenCalled();
  });

  it("keeps the dialog open and explains the rows that failed", async () => {
    mockApi.applySubscriptionSuggestions.mockResolvedValue({
      applied: 1,
      results: [
        { id: "a1", status: "applied", ruleId: "r1", collisions: 0 },
        {
          id: "a2",
          status: "failed",
          ruleId: "",
          errorCode: "target_not_found",
          errorReason: "",
          errorAlias: "codex",
          collisions: 0,
        },
      ],
    });
    mockApi.listSubscriptionSuggestions
      .mockResolvedValueOnce([claude, skill])
      .mockResolvedValue([{ ...claude, applied: true }, skill]);
    vi.spyOn(appState, "afterRuleChange").mockImplementation(() => {});
    const addToast = vi.spyOn(appState, "addToast");
    const { findByRole, getByRole, findByText, onclose } = renderModal();

    await findByRole("button", { name: "Apply 2 rules" });
    await fireEvent.click(getByRole("button", { name: "Choose folder" }));
    await waitFor(() =>
      expect((getByRole("button", { name: "Apply 2 rules" }) as HTMLButtonElement).disabled).toBe(
        false
      )
    );
    await fireEvent.click(getByRole("button", { name: "Apply 2 rules" }));

    expect(await findByText("That folder cannot be used for codex.")).toBeDefined();
    expect(onclose).not.toHaveBeenCalled();
    expect(addToast).toHaveBeenCalledWith("success", "1 of 2 rules created. Check the rest.");
    await waitFor(() => expect(getByRole("button", { name: "Apply 1 rule" })).toBeDefined());
  });

  it("shows an empty state when the repository suggests nothing", async () => {
    mockApi.listSubscriptionSuggestions.mockResolvedValue([]);
    const { findByText } = renderModal();
    expect(await findByText("This repository has no suggested rules.")).toBeDefined();
  });
});

describe("SubscriptionCard.svelte suggestions control", () => {
  const base = {
    id: "s_1",
    name: "Team skills",
    url: "https://example.com/team/skills.git",
    branch: "main",
    autoUpdate: false,
    addedAt: "2026-10-30T10:00:00Z",
    ruleCount: 1,
    suggestionsPending: 0,
  };
  const handlers = {
    oncheck: () => {},
    onupdate: () => {},
    onautoupdate: () => {},
    onbrowse: () => {},
    onremove: () => {},
    onviewdiff: () => {},
  };

  beforeEach(() => setLocale("en"));
  afterEach(() => cleanup());

  it("is hidden when nothing is pending", () => {
    const { queryByRole } = render(SubscriptionCard, {
      subscription: base as never,
      onsuggestions: () => {},
      ...handlers,
    });
    expect(queryByRole("button", { name: /suggested rule/ })).toBeNull();
  });

  it("shows the pending count without adding a row to the card", async () => {
    const onsuggestions = vi.fn();
    const { getByRole, container } = render(SubscriptionCard, {
      subscription: { ...base, suggestionsPending: 3 } as never,
      onsuggestions,
      ...handlers,
    });
    const control = getByRole("button", { name: "3 suggested rules not applied yet" });
    expect(control.textContent?.trim()).toBe("3");
    expect(container.querySelector(".actions")?.contains(control)).toBe(false);
    await fireEvent.click(control);
    expect(onsuggestions).toHaveBeenCalled();
  });

  it("uses the singular label for one suggestion", () => {
    const { getByRole } = render(SubscriptionCard, {
      subscription: { ...base, suggestionsPending: 1 } as never,
      onsuggestions: () => {},
      ...handlers,
    });
    expect(getByRole("button", { name: "1 suggested rule not applied yet" })).toBeDefined();
  });
});

describe("SubscriptionsView.svelte suggestions", () => {
  const added = {
    id: "s_9",
    name: "Morphy-Sync",
    url: "https://example.com/m.git",
    branch: "main",
    autoUpdate: false,
    addedAt: "2026-10-30T10:00:00Z",
    ruleCount: 0,
    suggestionsPending: 1,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    mockApi.listSubscriptions.mockResolvedValue([]);
    mockApi.addSubscription.mockResolvedValue(added);
    mockApi.getSubscriptionTree.mockResolvedValue([]);
  });

  afterEach(() => cleanup());

  async function addFromUrl(view: ReturnType<typeof render>) {
    await fireEvent.click((await view.findAllByText("Add subscription"))[0]);
    await fireEvent.input(await view.findByLabelText("Repository URL"), {
      target: { value: "https://example.com/m.git" },
    });
    await fireEvent.click(view.getByRole("button", { name: "Add" }));
  }

  it("opens the suggestions right after adding a subscription that has some", async () => {
    mockApi.listSubscriptionSuggestions.mockResolvedValue([claude]);
    const view = render(SubscriptionsView);
    await addFromUrl(view);
    expect(await view.findByText("Suggested rules · Morphy-Sync")).toBeDefined();
    expect(mockApi.listSubscriptionSuggestions).toHaveBeenCalledWith("s_9");
  });

  it("goes straight to browsing when the repository has no usable suggestions", async () => {
    mockApi.listSubscriptionSuggestions.mockResolvedValue([applied, invalid]);
    mockApi.listSubscriptions.mockResolvedValueOnce([]).mockResolvedValue([added]);
    const view = render(SubscriptionsView);
    await addFromUrl(view);
    await waitFor(() => expect(mockApi.getSubscriptionTree).toHaveBeenCalledWith("s_9"));
    expect(view.queryByText("Suggested rules · Morphy-Sync")).toBeNull();
  });
});
