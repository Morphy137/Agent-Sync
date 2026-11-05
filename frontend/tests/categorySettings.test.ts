import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, waitFor } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";
import { appState } from "../src/lib/stores/appState.svelte";

const mockApi = vi.hoisted(() => ({
  getRuleCategories: vi.fn(),
  getRulesReport: vi.fn(),
  saveRuleCategory: vi.fn(),
  deleteRuleCategory: vi.fn(),
}));
vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));
import CategorySettings from "../src/lib/components/rules/CategorySettings.svelte";

describe("category settings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setLocale("en");
    appState.config = { rootPath: "C:/repo" } as any;
    mockApi.getRulesReport.mockResolvedValue({ rules: [], collisions: [], unmapped: [] });
    mockApi.getRuleCategories.mockResolvedValue([
      { id: "skill", name: "", scope: "builtin", color: "purple" },
      { id: "custom", name: "Work", scope: "local", color: "blue" },
    ]);
  });
  afterEach(cleanup);

  it("previews the chosen color, icon and name before saving", async () => {
    const view = render(CategorySettings, { open: true });
    await fireEvent.click(await view.findByRole("button", { name: "Edit category: Skills" }));
    await fireEvent.input(view.getByLabelText("Name"), { target: { value: "My skills" } });
    const preview = view.getByText("My skills").closest(".preview")!;
    const previewSection = preview.parentElement!;
    expect(view.getByText("Example rule").classList.contains("category-rule-surface")).toBe(true);
    expect(previewSection.getAttribute("style")).toContain("--category-purple");
    await fireEvent.click(view.getByRole("button", { name: "Rose" }));
    expect(view.getByRole("button", { name: "Rose" }).getAttribute("aria-pressed")).toBe("true");
    expect(previewSection.getAttribute("style")).toContain("--category-rose");
    await fireEvent.click(view.getByRole("button", { name: "Star" }));
    expect(view.getByRole("button", { name: "Star" }).getAttribute("aria-pressed")).toBe("true");
    expect(preview.querySelector(".lucide-star")).not.toBeNull();
    expect(mockApi.saveRuleCategory).not.toHaveBeenCalled();
    await fireEvent.click(view.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(mockApi.saveRuleCategory).toHaveBeenCalled());
    expect(mockApi.saveRuleCategory.mock.calls[0][0]).toMatchObject({
      name: "My skills",
      color: "rose",
      icon: "star",
    });
  });

  it("edits builtin categories and confirms deletion of custom categories", async () => {
    const view = render(CategorySettings, { open: true });
    await fireEvent.click(await view.findByRole("button", { name: "Edit category: Skills" }));
    await fireEvent.input(view.getByLabelText("Name"), { target: { value: "My skills" } });
    await fireEvent.click(view.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(mockApi.saveRuleCategory).toHaveBeenCalled());
    expect(mockApi.saveRuleCategory.mock.calls[0][0]).toMatchObject({
      id: "skill",
      name: "My skills",
    });
    expect(mockApi.saveRuleCategory.mock.calls[0][1]).toBe("repo");

    await fireEvent.click(await view.findByRole("button", { name: "Delete category: Skills" }));
    expect(view.getByText("Rules in Skills will move to Uncategorized.")).toBeDefined();
    await fireEvent.click(view.getByRole("button", { name: "Delete category", exact: true }));
    await waitFor(() => expect(mockApi.deleteRuleCategory).toHaveBeenCalledWith("skill", "repo"));

    await fireEvent.click(await view.findByRole("button", { name: "Delete category: Work" }));
    expect(view.getByText("Rules in Work will move to Uncategorized.")).toBeDefined();
    await fireEvent.click(view.getByRole("button", { name: "Delete category", exact: true }));
    await waitFor(() => expect(mockApi.deleteRuleCategory).toHaveBeenCalledWith("custom", "local"));
  });
});
