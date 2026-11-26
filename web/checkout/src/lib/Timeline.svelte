<script lang="ts">
  type Ev = { step: string; status: string; detail?: string };
  let { events }: { events: Ev[] } = $props();

  const steps = [
    { key: 'RESERVE', label: 'Funds reserved' },
    { key: 'RISK', label: 'Risk check' },
    { key: 'DEBIT', label: 'Debited' },
    { key: 'SETTLE', label: 'Settled' }
  ];

  function stateOf(key: string): 'done' | 'retry' | 'failed' | 'pending' {
    const last = [...events].reverse().find((e) => e.step === key && e.status !== 'RELEASED');
    if (!last) return 'pending';
    if (last.status === 'OK') return 'done';
    if (last.status === 'RETRY') return 'retry';
    return 'failed';
  }
  const mark = { done: '✓', retry: '↻', failed: '✕', pending: '·' };
  const text = { done: 'complete', retry: 'retrying', failed: 'failed', pending: 'waiting' };
</script>

<ol class="timeline" aria-live="polite" aria-label="Payment progress">
  {#each steps as s (s.key)}
    {@const st = stateOf(s.key)}
    <li class={st}>
      <span class="mark" aria-hidden="true">{mark[st]}</span>
      <span class="label">{s.label}</span>
      <span class="sr">{text[st]}</span>
      {#if st === 'retry'}<small class="hint">Bank busy, retrying automatically…</small>{/if}
    </li>
  {/each}
</ol>

<style>
  .timeline { list-style: none; padding: 0; margin: 1rem 0; display: grid; gap: 0.5rem; }
  li { display: grid; grid-template-columns: 2rem 1fr; align-items: center; padding: 0.75rem; border-radius: 0.5rem; background: #f1f5f9; color: #334155; }
  li.done { background: #dcfce7; color: #14532d; }
  li.retry { background: #fef3c7; color: #78350f; }
  li.failed { background: #fee2e2; color: #7f1d1d; }
  .mark { font-weight: 700; }
  .hint { grid-column: 2; }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
</style>
