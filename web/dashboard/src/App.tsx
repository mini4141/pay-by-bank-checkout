import { ConnectError } from '@connectrpc/connect';
import type { Checkout } from '@pay/gen/checkout_pb';
import type { Payment } from '@pay/gen/common_pb';
import QRCode from 'qrcode';
import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { createApi, formatMoney, type Api } from './api';
import { PaymentsTable } from './PaymentsTable';

const KEY = 'merchant-api-key';

export function App() {
  const [apiKey, setApiKey] = useState(() => localStorage.getItem(KEY) ?? '');
  if (!apiKey) {
    return (
      <Login
        onSubmit={(k) => {
          localStorage.setItem(KEY, k);
          setApiKey(k);
        }}
      />
    );
  }
  return (
    <Dashboard
      apiKey={apiKey}
      onLogout={() => {
        localStorage.removeItem(KEY);
        setApiKey('');
      }}
    />
  );
}

function Login({ onSubmit }: { onSubmit: (key: string) => void }) {
  const [key, setKey] = useState('');
  return (
    <form
      className="login"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(key.trim());
      }}
    >
      <h1>Merchant dashboard</h1>
      <label>
        API key
        <input value={key} onChange={(e) => setKey(e.target.value)} placeholder="sk_live_…" autoFocus />
      </label>
      <button type="submit">Sign in</button>
    </form>
  );
}

/** Live payments: the stream ends every ~20s server-side, so re-subscribe in a loop. */
function usePayments(api: Api) {
  const [payments, setPayments] = useState<Payment[]>([]);
  const [error, setError] = useState('');
  useEffect(() => {
    const ac = new AbortController();
    (async () => {
      while (!ac.signal.aborted) {
        try {
          for await (const msg of api.payment.watchMerchantPayments({}, { signal: ac.signal })) {
            setPayments(msg.payments);
            setError('');
          }
        } catch (e) {
          if (ac.signal.aborted) return;
          setError(ConnectError.from(e).message);
          await new Promise((r) => setTimeout(r, 1500));
        }
      }
    })();
    return () => ac.abort();
  }, [api]);
  return { payments, error };
}

function useBalance(api: Api, payments: Payment[]) {
  const [balance, setBalance] = useState<{ name: string; text: string }>({ name: '', text: '…' });
  // Refresh whenever the payments change (a settle or refund moves the ledger).
  useEffect(() => {
    api.merchant.getBalance({}).then((r) => setBalance({ name: r.merchantName, text: formatMoney(r.balance!.amountMinor, r.balance!.currency) }));
  }, [api, payments]);
  return balance;
}

function Dashboard({ apiKey, onLogout }: { apiKey: string; onLogout: () => void }) {
  const api = useMemo(() => createApi(apiKey), [apiKey]);
  const { payments, error } = usePayments(api);
  const balance = useBalance(api, payments);
  const [selectedId, setSelectedId] = useState('');
  const selected = payments.find((p) => p.id === selectedId);

  return (
    <div className="app">
      <header>
        <h1>{balance.name || 'Merchant'} · dashboard</h1>
        <div className="tile" aria-label="Balance">
          <span className="muted">Balance</span> <strong>{balance.text}</strong>
        </div>
        <button className="link" onClick={onLogout}>
          Sign out
        </button>
      </header>
      {error && <p role="alert" className="error">{error}</p>}
      <div className="grid">
        <section>
          <NewCheckout api={api} />
          <h2>Payments</h2>
          <PaymentsTable payments={payments} selectedId={selectedId} onSelect={setSelectedId} />
        </section>
        <section>{selected ? <Detail api={api} payment={selected} /> : <p className="muted">Select a payment to see its timeline and webhook deliveries.</p>}</section>
      </div>
    </div>
  );
}

function NewCheckout({ api }: { api: Api }) {
  const [amount, setAmount] = useState('1200');
  const [reference, setReference] = useState('order-1001');
  const [created, setCreated] = useState<Checkout | null>(null);
  const [qr, setQr] = useState('');
  const [error, setError] = useState('');

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError('');
    try {
      const r = await api.checkout.createCheckout({
        amount: { amountMinor: BigInt(amount), currency: 'JPY' },
        reference,
        returnUrl: window.location.origin,
        idempotencyKey: crypto.randomUUID()
      });
      setCreated(r.checkout ?? null);
      setQr(await QRCode.toDataURL(r.checkout!.checkoutUrl, { margin: 1, width: 160 }));
    } catch (err) {
      setError(ConnectError.from(err).message);
    }
  }

  return (
    <div className="card">
      <h2>New checkout</h2>
      <form onSubmit={submit} className="inline">
        <label>
          Amount (¥)
          <input type="number" min="1" value={amount} onChange={(e) => setAmount(e.target.value)} />
        </label>
        <label>
          Reference
          <input value={reference} onChange={(e) => setReference(e.target.value)} />
        </label>
        <button type="submit">Create checkout</button>
      </form>
      {error && <p role="alert" className="error">{error}</p>}
      {created && (
        <div className="created">
          {qr && <img src={qr} alt="Checkout QR code" width={160} height={160} />}
          <div>
            <span className="muted">Checkout URL</span>
            <p>
              <a href={created.checkoutUrl} target="_blank" rel="noreferrer" data-testid="checkout-url">
                {created.checkoutUrl}
              </a>
            </p>
          </div>
        </div>
      )}
    </div>
  );
}

const STEPS = ['RESERVE', 'RISK', 'DEBIT', 'SETTLE', 'REFUND'];

function Detail({ api, payment }: { api: Api; payment: Payment }) {
  const [error, setError] = useState('');
  async function refund() {
    setError('');
    try {
      await api.payment.refundPayment({ paymentId: payment.id, idempotencyKey: crypto.randomUUID() });
    } catch (e) {
      setError(ConnectError.from(e).message);
    }
  }
  return (
    <div className="card">
      <h2>
        Payment <span className="mono">{payment.id}</span>
      </h2>
      <p>
        <span className={`badge ${payment.status.toLowerCase()}`}>{payment.status}</span>{' '}
        <span className="muted">workflow</span> <span className="mono">{payment.workflowId}</span>
      </p>
      <button onClick={refund} disabled={payment.status !== 'SETTLED'}>
        Refund
      </button>
      {error && <p role="alert" className="error">{error}</p>}
      <h3>Timeline</h3>
      <ul className="events">
        {payment.events
          .filter((e) => STEPS.includes(e.step))
          .map((e) => (
            <li key={e.id} className={e.status.toLowerCase()}>
              <strong>{e.step}</strong> {e.status} <span className="muted">{e.detail}</span>
            </li>
          ))}
      </ul>
      <h3>Webhook deliveries</h3>
      {payment.deliveries.length === 0 ? (
        <p className="muted">No deliveries yet.</p>
      ) : (
        <table aria-label="Webhook deliveries">
          <thead>
            <tr>
              <th>Event</th>
              <th>Attempt</th>
              <th>HTTP</th>
              <th>Error</th>
            </tr>
          </thead>
          <tbody>
            {payment.deliveries.map((d) => (
              <tr key={d.id}>
                <td>{d.eventType}</td>
                <td>{d.attempt}</td>
                <td>{d.statusCode}</td>
                <td>{d.error}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
