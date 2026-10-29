<script lang="ts">
  import { onMount } from "svelte";
  import { api } from "./lib/api";

  let loaded = $state(false);

  onMount(() => {
    api.getConfig().then(() => {
      loaded = true;
    });
  });
</script>

<main>
  {#if loaded}
    <p>AgentSync Ready</p>
  {/if}
</main>
