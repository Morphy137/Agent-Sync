/**
 * Moves an element to document.body so it is not affected by its parent's opacity,
 * transforms or stacking context (e.g. popups rendered from the title bar).
 */
export function portal(node: HTMLElement, target: HTMLElement = document.body) {
  target.appendChild(node);
  return {
    destroy() {
      node.remove();
    },
  };
}
