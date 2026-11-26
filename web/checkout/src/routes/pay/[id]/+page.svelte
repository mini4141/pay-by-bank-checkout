<script lang="ts">
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { ConnectError } from '@connectrpc/connect';
  import type { Checkout } from '@pay/gen/checkout_pb';
  import type { Payment } from '@pay/gen/common_pb';
  import { clients, formatMoney } from '$lib/client';
  import Timeline from '$lib/Timeline.svelte';

  const banks = [
    { id: 'sakura', name: 'Sakura Bank', color: '#be185d' },
    { id: 'fuji', name: 'Fuji Trust', color: '#1d4ed8' },
    { id: 'aoba', name: 'Aoba Bank', color: '#047857' }
  ];

  const api = clients();
  let checkout = $state<Checkout | null>(null);
  let payment = $state<Payment | null>(null);
  let step = $state<'bank' | 'consent' | 'confirm' | 'progress'>('bank');
  let bankId = $state('sakura');
  let busy = $state(false);
  let error = $state('');
  const bank = $derived(banks.find((b) => b.id === bankId)!);
  const status = $derived(payment?.status ?? 'PENDING');

  onMount(async () => {
    try {
      checkout = (await api.checkout.getCheckout({ checkoutId: page.params.id })).checkout ?? null;
      if (checkout?.paymentId) {
        step = 'progress';
        watch(checkout.paymentId);
      }
    } catch (e) {
      error = ConnectError.from(e).message;
    }
  });

  async function pay() {
    busy = true;
    error = '';
    try {
      const r = await api.payment.startPayment({ checkoutId: checkout!.id, bank: bankId });
      payment = r.payment ?? null;
      step = 'progress';
      await watch(payment!.id);
    } catch (e) {
      error = ConnectError.from(e).message;
    } finally {
      busy = false;
    }
  }

  /** Re-subscribes until the payment leaves PENDING (the server ends streams after ~20s). */
  async function watch(paymentId: string) {
    while (true) {
      try {
        for await (const msg of api.payment.watchPayment({ paymentId })) payment = msg.payment ?? payment;
      } catch (e) {
        error = ConnectError.from(e).message;
        await new Promise((r) => setTimeout(r, 1000));
      }
      if (payment && payment.status !== 'PENDING') return;
    }
  }
</script>

{#if error}<p role="alert" class="error">{error}</p>{/if}

{#if checkout}
  <section class="summary" aria-label="Order summary">
    <p class="merchant">{checkout.merchantName}</p>
    <p class="amount">{formatMoney(checkout.amount!.amountMinor, checkout.amount!.currency)}</p>
    {#if checkout.reference}<p class="ref">Order {checkout.reference}</p>{/if}
  </section>

  {#if step === 'bank'}
    <form
      onsubmit={(e) => {
        e.preventDefault();
        step = 'consent';
      }}
    >
      <fieldset>
        <legend>Choose your bank</legend>
        {#each banks as b (b.id)}
          <label class="bank">
            <input type="radio" name="bank" value={b.id} bind:group={bankId} />
            <span class="swatch" style:background={b.color}></span>
            {b.name}
          </label>
        {/each}
      </fieldset>
      <button type="submit">Continue</button>
    </form>
  {:else if step === 'consent'}
    <section class="bankpage" style:border-color={bank.color} aria-label="{bank.name} consent">
      <h2 style:color={bank.color}>{bank.name}</h2>
      <p>
        <strong>{checkout.merchantName}</strong> requests permission to debit
        <strong>{formatMoney(checkout.amount!.amountMinor, checkout.amount!.currency)}</strong> once from your current account ••••1234.
      </p>
      <div class="row">
        <button class="secondary" onclick={() => (step = 'bank')}>Cancel</button>
        <button onclick={() => (step = 'confirm')} style:background={bank.color}>Approve</button>
      </div>
    </section>
  {:else if step === 'confirm'}
    <section aria-label="Confirm payment">
      <p>Pay from <strong>{bank.name}</strong></p>
      <button onclick={pay} disabled={busy}>Pay {formatMoney(checkout.amount!.amountMinor, checkout.amount!.currency)}</button>
    </section>
  {:else}
    <section aria-label="Payment status">
      <h2>{status === 'SETTLED' ? 'Payment complete' : status === 'PENDING' ? 'Processing your payment…' : `Payment ${status.toLowerCase()}`}</h2>
      <Timeline events={payment?.events ?? []} />
      {#if status === 'SETTLED' || status === 'REFUNDED'}
        <p role="status" class="ok">Settled. You can close this page.</p>
        {#if checkout.returnUrl}<a href={checkout.returnUrl}>Return to merchant</a>{/if}
      {/if}
    </section>
  {/if}
{:else if !error}
  <p>Loading…</p>
{/if}

<style>
  .summary { background: white; border-radius: 0.75rem; padding: 1rem; margin-bottom: 1rem; text-align: center; box-shadow: 0 1px 3px #0001; }
  .merchant { margin: 0; color: #475569; }
  .amount { margin: 0.25rem 0; font-size: 2rem; font-weight: 700; }
  .ref { margin: 0; color: #475569; font-size: 0.875rem; }
  fieldset { border: 0; padding: 0; margin: 0 0 1rem; display: grid; gap: 0.5rem; }
  legend { font-weight: 600; margin-bottom: 0.5rem; }
  .bank { display: flex; align-items: center; gap: 0.75rem; padding: 0.75rem; background: white; border-radius: 0.5rem; min-height: 48px; }
  .swatch { width: 1rem; height: 1rem; border-radius: 50%; }
  form button, section > button { width: 100%; }
  .bankpage { background: white; border-left: 6px solid; border-radius: 0.5rem; padding: 1rem; }
  .row { display: flex; gap: 0.5rem; }
  .row button { flex: 1; }
  .error { background: #fee2e2; color: #7f1d1d; padding: 0.75rem; border-radius: 0.5rem; }
  .ok { color: #14532d; font-weight: 600; }
</style>
