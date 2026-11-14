import { describe, it, expect } from 'vitest';
import { participantStatusKey, paidCount } from './participantStatus';
import type { OpenPlayParticipant } from '@/shared/types/api.types';

function participant(overrides: Partial<OpenPlayParticipant> = {}): OpenPlayParticipant {
  return {
    id: 'r1',
    first_name: 'Ana',
    last_name: 'Gomez',
    phone: '+5491155550000',
    status: 'confirmed',
    price: 500000,
    service_fee: 20000,
    created_at: '2026-11-14T12:00:00Z',
    payment: { status: 'deposit_paid', amount: 520000, service_fee: 20000, refund_amount: 0, method: 'mercadopago' },
    ...overrides,
  };
}

describe('participantStatusKey', () => {
  it('a spot still being paid is held', () => {
    expect(participantStatusKey(participant({ status: 'held', payment: null }))).toBe('held');
  });

  it('a paid spot is confirmed', () => {
    expect(participantStatusKey(participant())).toBe('confirmed');
  });

  it('a cancelled spot that never carried money is cancelledUnpaid', () => {
    expect(participantStatusKey(participant({ status: 'cancelled', payment: null }))).toBe('cancelledUnpaid');
  });

  it.each([
    ['refunded', 'refunded'],
    ['refund_pending', 'refundPending'],
    ['partial_refund', 'partialRefund'],
  ] as const)('a cancelled spot whose payment is %s reads %s', (paymentStatus, key) => {
    const payment = {
      status: paymentStatus,
      amount: 520000,
      service_fee: 20000,
      refund_amount: 520000,
      method: 'mercadopago' as const,
    };
    expect(participantStatusKey(participant({ status: 'cancelled', payment }))).toBe(key);
  });
});

describe('paidCount', () => {
  it('counts the confirmed spots, the ones whose payment a cancel would refund', () => {
    const list = [
      participant({ id: 'a' }),
      participant({ id: 'b', status: 'held', payment: null }),
      participant({ id: 'c' }),
      participant({ id: 'd', status: 'cancelled', payment: null }),
    ];
    expect(paidCount(list)).toBe(2);
  });
});
