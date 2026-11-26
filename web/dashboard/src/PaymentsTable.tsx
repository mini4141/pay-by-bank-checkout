import type { Payment } from '@pay/gen/common_pb';
import { formatMoney } from './api';

type Props = {
  payments: Payment[];
  selectedId?: string;
  onSelect: (id: string) => void;
};

export function PaymentsTable({ payments, selectedId, onSelect }: Props) {
  if (payments.length === 0) return <p className="muted">No payments yet.</p>;
  return (
    <table>
      <thead>
        <tr>
          <th>Reference</th>
          <th>Bank</th>
          <th className="num">Amount</th>
          <th>Status</th>
          <th>Payment</th>
        </tr>
      </thead>
      <tbody>
        {payments.map((p) => (
          <tr key={p.id} className={p.id === selectedId ? 'selected' : ''} onClick={() => onSelect(p.id)}>
            <td>{p.reference}</td>
            <td>{p.bank}</td>
            <td className="num">{formatMoney(p.amount!.amountMinor, p.amount!.currency)}</td>
            <td>
              <span className={`badge ${p.status.toLowerCase()}`}>{p.status}</span>
            </td>
            <td className="mono">{p.id}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
