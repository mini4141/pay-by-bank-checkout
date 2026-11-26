<script lang="ts">
  let { children } = $props();
</script>

<main>
  <header><span class="logo" aria-hidden="true">⇄</span> <strong>Pay by bank</strong></header>
  {@render children()}
</main>

<style>
  :global(body) { margin: 0; font-family: system-ui, sans-serif; background: #f8fafc; color: #0f172a; }
  :global(button) { font: inherit; min-height: 48px; border-radius: 0.5rem; border: 0; padding: 0 1rem; cursor: pointer; background: #1d4ed8; color: white; }
  :global(button.secondary) { background: #e2e8f0; color: #0f172a; }
  :global(button:disabled) { opacity: 0.6; cursor: wait; }
  :global(:focus-visible) { outline: 3px solid #f59e0b; outline-offset: 2px; }
  main { max-width: 28rem; margin: 0 auto; padding: 1rem; }
  header { padding: 0.5rem 0 1rem; display: flex; gap: 0.5rem; align-items: center; }
  .logo { font-size: 1.5rem; }
</style>
