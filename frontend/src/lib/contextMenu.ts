const EDITABLE_SELECTOR = "input, textarea, [contenteditable='true']";

/**
 * Blocks the browser context menu (Reload, Inspect, Save as...) so the window behaves like a
 * desktop app. Editable fields and selected text keep it for cut/copy/paste.
 */
export function shouldAllowContextMenu(target: EventTarget | null, selection: string): boolean {
  if (selection.trim()) return true;
  return target instanceof Element && Boolean(target.closest(EDITABLE_SELECTOR));
}

export function installContextMenuGuard(doc: Document = document): () => void {
  const handler = (event: MouseEvent) => {
    const selection = doc.getSelection()?.toString() ?? "";
    if (!shouldAllowContextMenu(event.target, selection)) event.preventDefault();
  };
  doc.addEventListener("contextmenu", handler);
  return () => doc.removeEventListener("contextmenu", handler);
}
