import { env } from '$env/dynamic/public';
import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { CheckoutService } from '@pay/gen/checkout_pb';
import { PaymentService } from '@pay/gen/payment_pb';

function baseUrl(): string {
  const u = env.PUBLIC_API_URL || '/api';
  return u.startsWith('/') ? window.location.origin + u : u;
}

export function clients() {
  const transport = createConnectTransport({ baseUrl: baseUrl() });
  return { checkout: createClient(CheckoutService, transport), payment: createClient(PaymentService, transport) };
}

/** Formats minor units using the currency's own number of fraction digits. */
export function formatMoney(amountMinor: bigint | number, currency: string): string {
  const fmt = new Intl.NumberFormat('en', { style: 'currency', currency });
  const digits = fmt.resolvedOptions().maximumFractionDigits ?? 2;
  return fmt.format(Number(amountMinor) / 10 ** digits);
}
