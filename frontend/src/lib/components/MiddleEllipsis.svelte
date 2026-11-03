<script lang="ts">
  interface Props {
    text: string;
    title?: string;
  }

  let { text, title = text }: Props = $props();

  // The last segment never shrinks; the start shrinks and shows "..." only when it is cut.
  const splitAt = $derived(Math.max(text.lastIndexOf("/"), text.lastIndexOf("\\")));
  const start = $derived(splitAt > 0 ? text.slice(0, splitAt) : "");
  const end = $derived(splitAt > 0 ? text.slice(splitAt) : text);
</script>

<span class="middle-ellipsis" {title}
  ><span class="start">{start}</span><span class="end">{end}</span></span
>

<style>
  .middle-ellipsis {
    display: flex;
    min-width: 0;
    white-space: nowrap;
  }

  .start {
    min-width: 0;
    flex: 0 1 auto;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .end {
    min-width: 0;
    flex: 0 0 auto;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
