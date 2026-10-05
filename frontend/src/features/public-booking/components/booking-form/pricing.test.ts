import { describe, it, expect } from 'vitest';
import { computeBookingPricing } from './pricing';
import type { BookingSlotInfo } from './types';

const baseSlotInfo: BookingSlotInfo = {
  complexId: 'c1',
  complexName: 'Club Norte',
  complexPhone: '1155550000',
  courtId: 'ct1',
  courtName: 'Cancha 1',
  date: '2026-03-20',
  startTime: '10:00',
  endTime: '11:30',
  durationMinutes: 90,
  price: 1_000_000,
  depositPercentage: 30,
  cancellationHours: 24,
};

describe('computeBookingPricing', () => {
  it('computes deposit amount from the percentage', () => {
    const pricing = computeBookingPricing(baseSlotInfo);
    expect(pricing.depositAmount).toBe(300_000);
    expect(pricing.hasDeposit).toBe(true);
    expect(pricing.mpAmount).toBe(300_000);
  });

  it('falls back to full price as mpAmount when depositPercentage is 0', () => {
    const pricing = computeBookingPricing({ ...baseSlotInfo, depositPercentage: 0 });
    expect(pricing.hasDeposit).toBe(false);
    expect(pricing.mpAmount).toBe(1_000_000);
  });

  it('uses the backend-provided serviceFee when present', () => {
    const pricing = computeBookingPricing({ ...baseSlotInfo, serviceFee: 12_345 });
    expect(pricing.serviceFee).toBe(12_345);
  });

  it('falls back to the flat 100000 centavos fee when serviceFee is absent', () => {
    const pricing = computeBookingPricing(baseSlotInfo);
    expect(pricing.serviceFee).toBe(100_000);
  });

  it('keeps the flat fallback fee whatever the online amount is', () => {
    const small = computeBookingPricing({ ...baseSlotInfo, price: 10_000, depositPercentage: 100 });
    const large = computeBookingPricing({ ...baseSlotInfo, price: 10_000_000, depositPercentage: 100 });
    expect(small.serviceFee).toBe(100_000);
    expect(large.serviceFee).toBe(100_000);
  });

  it('computes totalOnline as mpAmount + serviceFee', () => {
    const pricing = computeBookingPricing({ ...baseSlotInfo, serviceFee: 50_000 });
    expect(pricing.totalOnline).toBe(350_000);
  });

  it('computes remainingAmount as price - mpAmount', () => {
    const pricing = computeBookingPricing(baseSlotInfo);
    expect(pricing.remainingAmount).toBe(700_000);
  });
});
