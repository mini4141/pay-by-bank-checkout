import { expect, test, type Page } from '@playwright/test';
import { mkdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

// All endpoints are configurable so the same spec runs locally and against AWS.
const env = process.env;
const DASHBOARD_URL = env.DASHBOARD_URL ?? 'http://localhost:5173';
const API_URL = env.API_URL ?? 'http://localhost:8080/api';
const RECEIVER_URL = env.RECEIVER_URL ?? 'http://localhost:8082/receiver';
const TEMPORAL_UI_URL = env.TEMPORAL_UI_URL ?? 'http://localhost:8233';
const MOCKBANK_URL = env.MOCKBANK_URL ?? (env.CLOUD ? '' : 'http://localhost:8081'); // not exposed in the cloud
const SEED_FILE = env.SEED_FILE ?? '../.local/seed.json';
const SHOTS = env.PROOF_DIR ? join(env.PROOF_DIR, 'screenshots') : '';
const PREFIX = env.CLOUD ? 'cloud-' : '';

const apiKey = env.API_KEY ?? JSON.parse(readFileSync(SEED_FILE, 'utf8')).api_key;

async function shot(page: Page, name: string) {
  if (!SHOTS) return;
  mkdirSync(SHOTS, { recursive: true });
  await page.screenshot({ path: join(SHOTS, `${name}.png`), fullPage: true });
}

test('demo story: merchant creates checkout, shopper pays, dashboard settles, webhook arrives, refund', async ({ browser }) => {
  // With a local mock bank, fail every debit (capped at 3 per payment) so retries are always visible.
  if (MOCKBANK_URL) {
    await fetch(`${MOCKBANK_URL}/admin/fail-rate`, { method: 'POST', body: JSON.stringify({ rate: 1 }) });
  }

  // 1. Merchant signs in and creates a ¥1,200 checkout.
  const merchant = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  await merchant.goto(DASHBOARD_URL);
  await merchant.getByLabel('API key').fill(apiKey);
  await merchant.getByRole('button', { name: 'Sign in' }).click();
  await expect(merchant.getByRole('heading', { name: /dashboard/ })).toBeVisible();
  const reference = `order-${Date.now()}`;
  await merchant.getByLabel('Amount (¥)').fill('1200');
  await merchant.getByLabel('Reference').fill(reference);
  await merchant.getByRole('button', { name: 'Create checkout' }).click();
  const link = merchant.getByTestId('checkout-url');
  await expect(link).toBeVisible();
  await expect(merchant.getByAltText('Checkout QR code')).toBeVisible();
  const checkoutUrl = (await link.textContent())!;
  await shot(merchant, `${PREFIX}01-dashboard-new-checkout`);
  const balanceBefore = await merchant.getByLabel('Balance').textContent();

  // 2. Shopper pays on a phone-sized viewport.
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const shopper = await ctx.newPage();
  await shopper.goto(checkoutUrl);
  await expect(shopper.getByText('Kissa Coffee')).toBeVisible();
  await expect(shopper.getByText('¥1,200')).toBeVisible();
  await shot(shopper, `${PREFIX}02-checkout-bank-select`);
  await shopper.getByLabel('Sakura Bank').check();
  await shopper.getByRole('button', { name: 'Continue' }).click();
  await expect(shopper.getByRole('heading', { name: 'Sakura Bank' })).toBeVisible();
  await shot(shopper, `${PREFIX}03-checkout-consent`);
  await shopper.getByRole('button', { name: 'Approve' }).click();
  await shopper.getByRole('button', { name: /^Pay / }).click();
  // The timeline completes without a refresh (WatchPayment stream).
  await expect(shopper.getByRole('heading', { name: 'Payment complete' })).toBeVisible({ timeout: 60_000 });
  await expect(shopper.getByText('Settled').first()).toBeVisible();
  await shot(shopper, `${PREFIX}04-checkout-timeline-settled`);

  // 3. The payment shows up live on the dashboard (no reload), settled, with its webhook delivery.
  const row = merchant.getByRole('row', { name: new RegExp(reference) });
  await expect(row).toBeVisible();
  await expect(row.getByText('SETTLED')).toBeVisible();
  await row.click();
  await expect(merchant.getByRole('table', { name: 'Webhook deliveries' }).getByRole('cell', { name: '204' })).toBeVisible();
  await expect(merchant.getByLabel('Balance')).not.toHaveText(balanceBefore!); // settled: balance went up
  await shot(merchant, `${PREFIX}05-dashboard-live-payment`);
  await merchant.getByRole('table', { name: 'Webhook deliveries' }).scrollIntoViewIfNeeded();
  await shot(merchant, `${PREFIX}06-webhook-deliveries`);

  // 4. The demo merchant receiver got a correctly signed webhook.
  const paymentId = (await row.locator('.mono').textContent())!;
  const deliveries = (await (await fetch(`${RECEIVER_URL}/deliveries`)).json()) as { valid: boolean; body: { payment_id: string; type: string } }[];
  expect(deliveries.some((d) => d.valid && d.body.payment_id === paymentId && d.body.type === 'payment.succeeded')).toBe(true);

  // 5. Temporal UI shows the workflow history (with the retried debit locally).
  const checkoutId = checkoutUrl.split('/').pop()!;
  const temporal = await ctx.newPage();
  await temporal.setViewportSize({ width: 1440, height: 1000 });
  await temporal.goto(`${TEMPORAL_UI_URL}/namespaces/default/workflows/payment-${checkoutId}`);
  await expect(temporal.getByText(`payment-${checkoutId}`).first()).toBeVisible({ timeout: 30_000 });
  await temporal.waitForTimeout(2000);
  await shot(temporal, `${PREFIX}07-temporal-ui-history-retries`);

  // 6. Refund from the dashboard: status REFUNDED, balance goes back down.
  await merchant.getByRole('button', { name: 'Refund' }).click();
  await expect(merchant.getByRole('row', { name: new RegExp(reference) }).getByText('REFUNDED')).toBeVisible({ timeout: 30_000 });
  // The refund reverses the ledger: the balance returns to what it was before the payment.
  await expect(merchant.getByLabel('Balance')).toHaveText(balanceBefore!);
  await shot(merchant, `${PREFIX}08-refunded`);

  if (MOCKBANK_URL) {
    await fetch(`${MOCKBANK_URL}/admin/fail-rate`, { method: 'POST', body: JSON.stringify({ rate: 0.5 }) });
  }
});
