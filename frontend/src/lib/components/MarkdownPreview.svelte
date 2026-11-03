<script lang="ts">
  import { marked } from "marked";
  import DOMPurify from "dompurify";
  import { api } from "../api";

  interface Props {
    /** Markdown source to render. */
    source: string;
  }

  /** YAML metadata at the top of a file (skills use it); Markdown would render it as a heading. */
  const FRONTMATTER_PATTERN = /^---\r?\n([\s\S]*?)\r?\n---[ \t]*(?:\r?\n|$)/;

  let { source }: Props = $props();

  const frontmatter = $derived(source.match(FRONTMATTER_PATTERN));

  const html = $derived(
    DOMPurify.sanitize(marked.parse(source.slice(frontmatter?.[0].length ?? 0), { async: false }), {
      USE_PROFILES: { html: true },
      FORBID_ATTR: ["style"],
    })
  );

  /** Links never navigate the app: web links open in the system browser, the rest are ignored. */
  function handleClick(event: MouseEvent) {
    const link = (event.target as Element).closest("a");
    if (!link) return;
    event.preventDefault();
    if (/^https?:\/\//i.test(link.getAttribute("href") ?? "")) api.openURL(link.href);
  }

  function links(node: HTMLElement) {
    node.addEventListener("click", handleClick);
    return { destroy: () => node.removeEventListener("click", handleClick) };
  }
</script>

<div class="markdown-preview" role="document" use:links>
  {#if frontmatter}<pre class="frontmatter">{frontmatter[1]}</pre>{/if}
  <!-- eslint-disable-next-line svelte/no-at-html-tags -- html is sanitized with DOMPurify -->
  {@html html}
</div>

<style>
  .frontmatter {
    margin: 0 0 var(--space-3);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-md);
    background: var(--surface);
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    white-space: pre-wrap;
  }

  .markdown-preview {
    flex: 1 1 auto;
    min-height: var(--markdown-preview-min-height, 0);
    overflow: auto;
    padding: var(--space-3);
    background: var(--surface-sunken);
    border-radius: var(--radius-md);
    color: var(--ink);
    line-height: 1.6;
    overflow-wrap: anywhere;
    user-select: text;
  }
  .markdown-preview :global(pre) {
    overflow-x: auto;
    padding: var(--space-3);
    background: var(--surface);
    border-radius: var(--radius-sm);
  }
  .markdown-preview :global(code) {
    font-family: var(--font-mono);
  }
  .markdown-preview :global(img) {
    max-width: 100%;
  }
  .markdown-preview :global(table) {
    display: block;
    overflow-x: auto;
    border-collapse: collapse;
  }
  .markdown-preview :global(th),
  .markdown-preview :global(td) {
    padding: var(--space-2);
    border: 1px solid var(--border-subtle);
  }
</style>
