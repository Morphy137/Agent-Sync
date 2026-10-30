import { diffLines } from "diff";

export type DiffRowKind = "context" | "added" | "removed";

export interface DiffRow {
  kind: DiffRowKind;
  oldLine: number | null;
  newLine: number | null;
  text: string;
}

export interface DiffSummary {
  rows: DiffRow[];
  added: number;
  removed: number;
  identical: boolean;
}

function splitLines(value: string): string[] {
  const lines = value.split("\n");
  if (lines.length > 0 && lines[lines.length - 1] === "") lines.pop();
  return lines.map((line) => line.replace(/\r$/, ""));
}

/** Builds unified diff rows (old = left, new = right) with line numbers for both sides. */
export function buildLineDiff(left: string, right: string): DiffSummary {
  const rows: DiffRow[] = [];
  let oldLine = 1;
  let newLine = 1;
  let added = 0;
  let removed = 0;

  for (const change of diffLines(left, right)) {
    for (const text of splitLines(change.value)) {
      if (change.added) {
        rows.push({ kind: "added", oldLine: null, newLine: newLine++, text });
        added++;
      } else if (change.removed) {
        rows.push({ kind: "removed", oldLine: oldLine++, newLine: null, text });
        removed++;
      } else {
        rows.push({ kind: "context", oldLine: oldLine++, newLine: newLine++, text });
      }
    }
  }
  return { rows, added, removed, identical: added === 0 && removed === 0 };
}
