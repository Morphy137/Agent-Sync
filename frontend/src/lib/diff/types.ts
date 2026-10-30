/** Mirrors rulesync.Diff from the Go backend. */
export interface FileDiff {
  leftLabel: string;
  rightLabel: string;
  left: string;
  right: string;
  binary: boolean;
  transform: boolean;
}

export interface DiffAction {
  label: string;
  variant?: "primary" | "secondary" | "ghost";
  disabled?: boolean;
  onclick: () => void;
}
