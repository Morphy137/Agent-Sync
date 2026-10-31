<script lang="ts">
  import { Sparkles, GitFork, MapPin, RefreshCw, Rss, CircleCheck } from "@lucide/svelte";
  import { t } from "../i18n";
  import { appState, type AppView } from "../stores/appState.svelte";
  import Button from "./Button.svelte";

  interface Props {
    open: boolean;
    onfinish: () => void;
  }

  let { open, onfinish }: Props = $props();

  interface Step {
    id: string;
    icon: typeof Sparkles;
    /** Tab shown behind the card so the user sees what the step talks about. */
    view?: AppView;
  }

  const STEPS: Step[] = [
    { id: "welcome", icon: Sparkles },
    { id: "rules", icon: GitFork, view: "rules" },
    { id: "locations", icon: MapPin, view: "locations" },
    { id: "sync", icon: RefreshCw },
    { id: "subscriptions", icon: Rss, view: "subscriptions" },
    { id: "done", icon: CircleCheck, view: "rules" },
  ];

  let index = $state(0);
  let wasOpen = false;

  const step = $derived(STEPS[index]);
  const isFirst = $derived(index === 0);
  const isLast = $derived(index === STEPS.length - 1);

  $effect(() => {
    if (open && !wasOpen) index = 0;
    wasOpen = open;
  });

  $effect(() => {
    if (open && step.view) appState.activeView = step.view;
  });

  function next() {
    if (isLast) onfinish();
    else index += 1;
  }

  function back() {
    if (!isFirst) index -= 1;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (!open) return;
    if (e.key === "Escape") onfinish();
    else if (e.key === "ArrowRight") next();
    else if (e.key === "ArrowLeft") back();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  <div class="tutorial-backdrop" role="presentation">
    <div
      class="tutorial-card"
      role="dialog"
      aria-modal="true"
      aria-label={t("tutorial.aria_label")}
      aria-describedby="tutorial-body"
    >
      <div class="tutorial-icon" aria-hidden="true">
        <step.icon size={28} />
      </div>

      <h2 class="tutorial-title">{t(`tutorial.steps.${step.id}.title`)}</h2>
      <p id="tutorial-body" class="tutorial-body">{t(`tutorial.steps.${step.id}.body`)}</p>

      <div
        class="tutorial-dots"
        role="progressbar"
        aria-valuemin={1}
        aria-valuemax={STEPS.length}
        aria-valuenow={index + 1}
        aria-label={t("tutorial.progress", { current: index + 1, total: STEPS.length })}
      >
        {#each STEPS as s, i (s.id)}
          <span class="dot" class:active={i === index}></span>
        {/each}
      </div>

      <div class="tutorial-actions">
        {#if isLast}
          <span></span>
        {:else}
          <Button variant="ghost" size="sm" onclick={onfinish}>{t("tutorial.skip")}</Button>
        {/if}
        <div class="nav">
          {#if !isFirst}
            <Button variant="secondary" size="sm" onclick={back}>{t("tutorial.back")}</Button>
          {/if}
          <Button variant="primary" size="sm" onclick={next}>
            {isLast ? t("tutorial.finish") : t("tutorial.next")}
          </Button>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .tutorial-backdrop {
    position: fixed;
    inset: 0;
    z-index: 200;
    display: flex;
    align-items: flex-end;
    justify-content: center;
    padding-bottom: 12vh;
    background-color: color-mix(in srgb, var(--backdrop-overlay) 60%, transparent);
  }

  .tutorial-card {
    width: min(420px, 90%);
    padding: var(--space-6);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    text-align: center;
    background-color: var(--surface);
    border-radius: var(--radius-xl);
    box-shadow: var(--elev-3);
    animation: tutorialEnter var(--duration-normal) var(--easing-out);
  }

  .tutorial-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 56px;
    height: 56px;
    border-radius: var(--radius-lg);
    background-color: color-mix(in srgb, var(--accent) 14%, transparent);
    color: var(--accent);
  }

  .tutorial-title {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .tutorial-body {
    margin: 0;
    min-height: 3em;
    font-size: var(--text-sm);
    line-height: 1.5;
    color: var(--text-muted);
  }

  .tutorial-dots {
    display: flex;
    gap: var(--space-2);
  }

  .dot {
    width: 6px;
    height: 6px;
    border-radius: var(--radius-full);
    background-color: var(--text-subtle);
    transition:
      width var(--duration-fast) var(--easing-out),
      background-color var(--duration-fast) var(--easing-out);
  }

  .dot.active {
    width: 18px;
    background-color: var(--accent);
  }

  .tutorial-actions {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: var(--space-2);
  }

  .nav {
    display: flex;
    gap: var(--space-2);
  }

  @keyframes tutorialEnter {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .tutorial-card {
      animation: none;
    }
  }
</style>
