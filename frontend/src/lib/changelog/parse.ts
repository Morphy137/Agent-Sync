/** Structured view of CHANGELOG.md ("Keep a Changelog" layout) for the in-app viewer. */
export interface ChangelogSection {
  /** Group name as written in the file: Added, Changed, Fixed, Removed... */
  title: string;
  items: string[];
}

export interface ChangelogRelease {
  /** Version without brackets ("0.0.1") or "Unreleased". */
  version: string;
  date: string;
  unreleased: boolean;
  sections: ChangelogSection[];
}

export const UNRELEASED = "Unreleased";

const RELEASE_HEADING = /^##\s+\[?([^\]\s]+)\]?(?:\s+-\s+(\S+))?\s*$/;
const SECTION_HEADING = /^###\s+(.+?)\s*$/;
const BULLET = /^\s*[-*]\s+(.+)$/;

export function parseChangelog(markdown: string): ChangelogRelease[] {
  const releases: ChangelogRelease[] = [];
  let release: ChangelogRelease | null = null;
  let section: ChangelogSection | null = null;

  for (const line of markdown.replace(/\r\n/g, "\n").split("\n")) {
    const releaseMatch = line.match(RELEASE_HEADING);
    if (releaseMatch) {
      const version = releaseMatch[1];
      release = {
        version,
        date: releaseMatch[2] ?? "",
        unreleased: version.toLowerCase() === UNRELEASED.toLowerCase(),
        sections: [],
      };
      releases.push(release);
      section = null;
      continue;
    }
    if (!release) continue;
    const sectionMatch = line.match(SECTION_HEADING);
    if (sectionMatch) {
      section = { title: sectionMatch[1], items: [] };
      release.sections.push(section);
      continue;
    }
    const bullet = line.match(BULLET);
    if (bullet && section) section.items.push(bullet[1].trim());
  }

  return releases.filter((r) => r.sections.some((s) => s.items.length > 0));
}

/** Latest published version, e.g. "0.0.1" ("" when nothing was released yet). */
export function latestVersion(releases: ChangelogRelease[]): string {
  return releases.find((r) => !r.unreleased)?.version ?? "";
}

export interface TextSegment {
  text: string;
  code: boolean;
}

/** Splits `inline code` and drops **bold** markers so items render as plain, safe text. */
export function toSegments(item: string): TextSegment[] {
  return item
    .replace(/\*\*(.+?)\*\*/g, "$1")
    .split(/(`[^`]+`)/)
    .filter(Boolean)
    .map((part) =>
      part.startsWith("`") && part.endsWith("`")
        ? { text: part.slice(1, -1), code: true }
        : { text: part, code: false }
    );
}
