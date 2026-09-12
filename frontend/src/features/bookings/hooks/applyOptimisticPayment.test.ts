import { describe, it, expect } from 'vitest';
import { applyOptimisticPayment } from './applyOptimisticPayment';
import { makeBooking } from '@/test/factories';
import type { Booking } from '@/shared/types/api.types';

const baseBooking = makeBooking({
  date: '2026-03-15',
  start_time: '10:00',
  duration_minutes: 90,
  price: 20000,
  client_name: 'Juan Perez',
  client_phone: '+541155550000',
});

describe('applyOptimisticPayment', () => {
  it('marks the booking fully_paid when the new payment covers the full price', () => {
    const result = applyOptimisticPayment(baseBooking, 20000);
    expect(result.collection_status).toBe('fully_paid');
    expect(result.deposit_amount).toBe(0);
  });

  it('marks the booking deposit_paid when the payment is partial', () => {
    const result = applyOptimisticPayment(baseBooking, 5000);
    expect(result.collection_status).toBe('deposit_paid');
    expect(result.deposit_amount).toBe(5000);
  });

  it('accumulates on top of an existing deposit when already deposit_paid', () => {
    const withDeposit: Booking = {
      ...baseBooking,
      collection_status: 'deposit_paid',
      deposit_amount: 5000,
    };
    const result = applyOptimisticPayment(withDeposit, 15000);
    // 5000 (existing deposit) + 15000 (new payment) = 20000 = full price
    expect(result.collection_status).toBe('fully_paid');
    expect(result.deposit_amount).toBe(5000);
  });

  it('stays deposit_paid when the accumulated total is still below price', () => {
    const withDeposit: Booking = {
      ...baseBooking,
      collection_status: 'deposit_paid',
      deposit_amount: 5000,
    };
    const result = applyOptimisticPayment(withDeposit, 3000);
    expect(result.collection_status).toBe('deposit_paid');
    expect(result.deposit_amount).toBe(8000);
  });
});
