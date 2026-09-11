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

  it('falls back to a 7% local estimate when serviceFee is absent', () => {
    const pricing = computeBookingPricing(baseSlotInfo);
    // 7% of 300000 = 21000, above the 100000 floor
    expect(pricing.serviceFee).toBe(100_000);
  });

  it('floors the local estimate at 100000 centavos', () => {
    const pricing = computeBookingPricing({
      ...baseSlotInfo,
      price: 10_000,
      depositPercentage: 100,
    });
    expect(pricing.serviceFee).toBe(100_000);
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
