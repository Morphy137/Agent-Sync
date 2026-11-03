/** True for file names that have a rendered preview (Markdown). */
export function supportsPreview(name: string): boolean {
  return /\.(md|markdown)$/i.test(name);
}
