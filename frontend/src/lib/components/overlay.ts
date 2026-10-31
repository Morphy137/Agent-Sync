const overlays: { node: HTMLElement; previous: HTMLElement | null }[] = [];

const focusable =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function overlay(node: HTMLElement, close: () => void) {
  const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const entry = { node, previous };
  overlays.push(entry);
  node.focus();

  function onKeydown(event: KeyboardEvent) {
    if (overlays.at(-1) !== entry || event.defaultPrevented) return;
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      close();
    } else if (event.key === "Tab") {
      const items = [...node.querySelectorAll<HTMLElement>(focusable)].filter(
        (item) =>
          item.tabIndex >= 0 &&
          !item.closest('[hidden], [inert], [aria-hidden="true"]') &&
          getComputedStyle(item).visibility !== "hidden" &&
          getComputedStyle(item).display !== "none"
      );
      const first = items[0] ?? node;
      const last = items.at(-1) ?? node;
      if (
        event.shiftKey &&
        (document.activeElement === first ||
          document.activeElement === node ||
          !node.contains(document.activeElement))
      ) {
        event.preventDefault();
        last.focus();
      } else if (
        !event.shiftKey &&
        (document.activeElement === last ||
          document.activeElement === node ||
          !node.contains(document.activeElement))
      ) {
        event.preventDefault();
        first.focus();
      }
    }
  }

  function onFocus(event: FocusEvent) {
    if (overlays.at(-1) === entry && !node.contains(event.target as Node)) node.focus();
  }

  document.addEventListener("keydown", onKeydown);
  document.addEventListener("focusin", onFocus);
  return {
    destroy() {
      const wasTop = overlays.at(-1) === entry;
      overlays.splice(overlays.indexOf(entry), 1);
      for (const item of overlays) {
        if (item.previous && node.contains(item.previous)) item.previous = entry.previous;
      }
      document.removeEventListener("keydown", onKeydown);
      document.removeEventListener("focusin", onFocus);
      if (wasTop) {
        if (
          entry.previous?.isConnected &&
          (!overlays.length || overlays.at(-1)?.node.contains(entry.previous))
        )
          entry.previous.focus();
        else overlays.at(-1)?.node.focus();
      }
    },
  };
}
