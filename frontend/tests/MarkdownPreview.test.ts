import { render } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";

vi.mock("../src/lib/api", () => ({ api: { openURL: vi.fn() } }));

import MarkdownPreview from "../src/lib/components/MarkdownPreview.svelte";

describe("MarkdownPreview", () => {
  it("shows YAML frontmatter as metadata instead of rendering it as a heading", () => {
    const source = "---\nname: docs-audit\ndescription: Audit docs\n---\n\n# Title\n";
    const { container } = render(MarkdownPreview, { source });

    expect(container.querySelector(".frontmatter")?.textContent).toBe(
      "name: docs-audit\ndescription: Audit docs"
    );
    const headings = [...container.querySelectorAll("h1, h2")].map((h) => h.textContent);
    expect(headings).toEqual(["Title"]);
    expect(container.querySelector("hr")).toBeNull();
  });

  it("handles Windows line endings in frontmatter", () => {
    const { container } = render(MarkdownPreview, { source: "---\r\nname: a\r\n---\r\n# T\r\n" });

    expect(container.querySelector(".frontmatter")?.textContent).toBe("name: a");
  });

  it("leaves a thematic break in the body alone", () => {
    const { container } = render(MarkdownPreview, { source: "Intro\n\n---\n\nEnd\n" });

    expect(container.querySelector(".frontmatter")).toBeNull();
    expect(container.querySelector("hr")).not.toBeNull();
  });
});
