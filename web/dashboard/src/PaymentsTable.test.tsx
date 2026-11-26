import { create } from '@bufbuild/protobuf';
import { MoneySchema, PaymentSchema } from '@pay/gen/common_pb';
import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PaymentsTable } from './PaymentsTable';

describe('PaymentsTable', () => {
  it('renders one row per payment with formatted amount and status', () => {
    const payments = [
      create(PaymentSchema, {
        id: 'pay_1',
        reference: 'order-1',
        bank: 'sakura',
        status: 'SETTLED',
        amount: create(MoneySchema, { amountMinor: 1200n, currency: 'JPY' })
      })
    ];
    render(<PaymentsTable payments={payments} onSelect={() => {}} />);
    expect(screen.getByText('order-1')).toBeTruthy();
    expect(screen.getByText('SETTLED')).toBeTruthy();
    expect(screen.getByText('¥1,200')).toBeTruthy();
  });
});
