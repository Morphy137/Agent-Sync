import type { service } from "../../../../wailsjs/go/models";

export type EntryKind = "skill" | "mcp" | "instructions" | "folder" | "file";

export interface TreeNode {
  name: string;
  path: string;
  isDir: boolean;
  kind: string;
  size: number;
  children: TreeNode[];
}

const SPECIFIC_KINDS: ReadonlySet<string> = new Set(["skill", "mcp", "instructions"]);

function compareNodes(a: TreeNode, b: TreeNode): number {
  if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
  return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
}

function sortRecursively(nodes: TreeNode[]): void {
  nodes.sort(compareNodes);
  for (const node of nodes) sortRecursively(node.children);
}

/** Builds a nested tree from flat, forward-slash paths (missing parent folders are synthesized). */
export function buildTree(entries: service.SourceTreeEntry[]): TreeNode[] {
  const root: TreeNode = { name: "", path: "", isDir: true, kind: "", size: 0, children: [] };
  const byPath = new Map<string, TreeNode>([["", root]]);

  const ensure = (path: string): TreeNode => {
    const existing = byPath.get(path);
    if (existing) return existing;
    const slash = path.lastIndexOf("/");
    const parent = ensure(slash >= 0 ? path.slice(0, slash) : "");
    const node: TreeNode = {
      name: path.slice(slash + 1),
      path,
      isDir: true,
      kind: "",
      size: 0,
      children: [],
    };
    parent.children.push(node);
    byPath.set(path, node);
    return node;
  };

  for (const entry of entries) {
    const path = entry.path.replace(/^\/+|\/+$/g, "");
    if (!path) continue;
    const node = ensure(path);
    node.isDir = entry.isDir;
    node.kind = entry.kind ?? "";
    node.size = entry.size ?? 0;
  }

  sortRecursively(root.children);
  return root.children;
}

/** Keeps nodes whose name/path matches the query, plus their ancestors and full subtrees. */
export function filterTree(nodes: TreeNode[], query: string): TreeNode[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return nodes;
  const result: TreeNode[] = [];
  for (const node of nodes) {
    if (node.path.toLowerCase().includes(needle)) {
      result.push(node);
      continue;
    }
    const children = filterTree(node.children, needle);
    if (children.length > 0) result.push({ ...node, children });
  }
  return result;
}

export function resolveKind(node: Pick<TreeNode, "isDir" | "kind">): EntryKind {
  if (SPECIFIC_KINDS.has(node.kind)) return node.kind as EntryKind;
  return node.isDir ? "folder" : "file";
}

export function countNodes(nodes: TreeNode[]): number {
  return nodes.reduce((total, node) => total + 1 + countNodes(node.children), 0);
}
