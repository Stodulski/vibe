import { manualRefundOwed } from './manualRefundOwed';
import type { Payment } from '@/shared/types/api.types';

const basePayment: Payment = {
  id: 'p1',
  booking_id: 'b1',
  complex_id: 'c1',
  amount: 10000,
  service_fee: 0,
  method: 'cash',
  status: 'fully_paid',
  refund_amount: 0,
  created_at: '2026-03-14T10:00:00Z',
  updated_at: '2026-03-14T10:00:00Z',
};

describe('manualRefundOwed', () => {
  it('sums the non-MP, non-refunded rows', () => {
    const payments: Payment[] = [
      { ...basePayment, id: 'p1', amount: 10000, service_fee: 500 },
      { ...basePayment, id: 'p2', method: 'transfer', amount: 5000, service_fee: 0 },
    ];
    expect(manualRefundOwed(payments)).toBe(15500);
  });

  it('excludes rows with an mp_payment_id', () => {
    const payments: Payment[] = [
      { ...basePayment, id: 'p1', amount: 10000 },
      { ...basePayment, id: 'p2', method: 'mercadopago', amount: 8000, mp_payment_id: 'mp-1' },
    ];
    expect(manualRefundOwed(payments)).toBe(10000);
  });

  it('excludes rows already marked refunded', () => {
    const payments: Payment[] = [
      { ...basePayment, id: 'p1', amount: 10000, status: 'refunded', refund_amount: 10000 },
      { ...basePayment, id: 'p2', amount: 5000 },
    ];
    expect(manualRefundOwed(payments)).toBe(5000);
  });

  it('subtracts a partial refund_amount already returned', () => {
    const payments: Payment[] = [{ ...basePayment, id: 'p1', amount: 10000, service_fee: 200, refund_amount: 4000 }];
    expect(manualRefundOwed(payments)).toBe(6200);
  });

  it('returns 0 when there is nothing outstanding', () => {
    expect(manualRefundOwed([])).toBe(0);
  });
});
