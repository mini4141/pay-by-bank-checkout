import { createClient, type Interceptor } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { CheckoutService } from '@pay/gen/checkout_pb';
import { MerchantService } from '@pay/gen/merchant_pb';
import { PaymentService } from '@pay/gen/payment_pb';

function baseUrl(): string {
  const u = (import.meta.env.VITE_API_URL as string | undefined) || '/api';
  return u.startsWith('/') ? window.location.origin + u : u;
}

export function createApi(apiKey: string) {
  const auth: Interceptor = (next) => (req) => {
    req.header.set('Authorization', `Bearer ${apiKey}`);
    return next(req);
  };
  const transport = createConnectTransport({ baseUrl: baseUrl(), interceptors: [auth] });
  return {
    merchant: createClient(MerchantService, transport),
    checkout: createClient(CheckoutService, transport),
    payment: createClient(PaymentService, transport)
  };
}
export type Api = ReturnType<typeof createApi>;

/** Formats minor units using the currency's own number of fraction digits. */
export function formatMoney(amountMinor: bigint | number, currency: string): string {
  const fmt = new Intl.NumberFormat('en', { style: 'currency', currency });
  const digits = fmt.resolvedOptions().maximumFractionDigits ?? 2;
  return fmt.format(Number(amountMinor) / 10 ** digits);
}

/** Idempotency key. crypto.randomUUID needs a secure context; getRandomValues works over plain HTTP too. */
export function newKey(): string {
  const b = crypto.getRandomValues(new Uint8Array(16));
  return Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
}
