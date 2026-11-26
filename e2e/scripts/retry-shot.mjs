// Screenshots the Temporal UI history of a payment whose debit was retried (found through the API).
// Usage: node retry-shot.mjs <api-url> <api-key> <temporal-ui-url> <out.png>
import { chromium } from '@playwright/test';

const [apiUrl, apiKey, temporalUrl, out] = process.argv.slice(2);

const res = await fetch(`${apiUrl}/pay.v1.PaymentService/ListPayments`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${apiKey}` },
  body: '{}'
});
const { payments = [] } = await res.json();
const retried = payments.find((p) => (p.events ?? []).some((e) => e.step === 'DEBIT' && e.status === 'RETRY'));
if (!retried) {
  console.log('no payment with a retried debit found; skipping');
  process.exit(0);
}
const retries = retried.events.filter((e) => e.step === 'DEBIT' && e.status === 'RETRY').length;
console.log(`payment ${retried.id}: ${retries} debit retries, workflow ${retried.workflowId}`);

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
await page.goto(`${temporalUrl}/namespaces/default/workflows/${retried.workflowId}`);
await page.getByText(retried.workflowId).first().waitFor({ timeout: 90_000 });
await page.waitForTimeout(2000);
await page.screenshot({ path: out, fullPage: true });
await browser.close();
