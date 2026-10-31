import { describe, it, expect, vi, afterEach } from "vitest";
import { render, cleanup } from "@testing-library/svelte";
import { setLocale } from "../src/lib/i18n";
import { latestVersion, parseChangelog, toSegments } from "../src/lib/changelog/parse";

const SAMPLE = `# Changelog

## [Unreleased]

### Added
- Pick several files at once.

### Fixed

## [0.0.1] - 2026-10-31

### Added
- Sync rules with \`AGENTS.md\`.
- **Locations** per PC.
`;

const mockApi = vi.hoisted(() => ({ getChangelog: vi.fn() }));
vi.mock("../src/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../src/lib/api")>()),
  api: mockApi,
}));

import ChangelogModal from "../src/lib/components/ChangelogModal.svelte";

describe("changelog parser", () => {
  it("reads releases, groups and bullets", () => {
    const releases = parseChangelog(SAMPLE);
    expect(releases.map((r) => r.version)).toEqual(["Unreleased", "0.0.1"]);
    expect(releases[0].unreleased).toBe(true);
    expect(releases[1].date).toBe("2026-10-31");
    expect(releases[1].sections[0].items).toHaveLength(2);
    expect(latestVersion(releases)).toBe("0.0.1");
  });

  it("turns inline code into segments and drops bold markers", () => {
    expect(toSegments("Sync rules with `AGENTS.md`.")).toEqual([
      { text: "Sync rules with ", code: false },
      { text: "AGENTS.md", code: true },
      { text: ".", code: false },
    ]);
    expect(toSegments("**Locations** per PC.")).toEqual([
      { text: "Locations per PC.", code: false },
    ]);
  });
});

describe("ChangelogModal.svelte", () => {
  afterEach(() => cleanup());

  it("shows translated groups and the upcoming version first", async () => {
    setLocale("es");
    mockApi.getChangelog.mockResolvedValue(SAMPLE);
    const { findByText, getAllByText } = render(ChangelogModal, { open: true, onclose: vi.fn() });
    expect(await findByText("En la próxima versión")).toBeDefined();
    expect(getAllByText("Nuevo")).toHaveLength(2);
    expect(await findByText("Versión 0.0.1")).toBeDefined();
    expect(await findByText("AGENTS.md")).toBeDefined();
  });
});
