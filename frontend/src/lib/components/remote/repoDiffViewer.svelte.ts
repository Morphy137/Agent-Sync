import { api, type RepoDiffMode } from "../../api";
import type { FileDiff } from "../../diff/types";
import { reportError } from "./reportError";

/** Loads repository diffs (working or incoming) for a DiffModal, ignoring stale responses. */
export class RepoDiffViewer {
  open = $state(false);
  loading = $state(false);
  diff = $state<FileDiff | null>(null);
  path = $state("");
  mode = $state<RepoDiffMode>("working");
  private requestId = 0;

  async show(path: string, mode: RepoDiffMode): Promise<void> {
    const request = ++this.requestId;
    this.path = path;
    this.mode = mode;
    this.diff = null;
    this.loading = true;
    this.open = true;
    try {
      const result = await api.getRepoFileDiff(path, mode);
      if (request === this.requestId) this.diff = result;
    } catch (err) {
      if (request === this.requestId) {
        this.open = false;
        reportError(err);
      }
    } finally {
      if (request === this.requestId) this.loading = false;
    }
  }

  close(): void {
    this.requestId++;
    this.open = false;
    this.loading = false;
    this.diff = null;
  }
}
