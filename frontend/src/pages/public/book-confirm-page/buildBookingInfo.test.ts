import { buildBookingInfo } from './buildBookingInfo';
import type { BookingSlotInfo } from '@/features/public-booking';

const slotInfo: BookingSlotInfo = {
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

describe('buildBookingInfo', () => {
  // The cached copy has to carry instants now: the confirmation screen renders
  // the hours from them, and its merge refuses to build a card without an end.
  // They are built on the venue's clock, not the runner's — 10:00 in Buenos
  // Aires is 13:00 UTC whatever machine this runs on.
  it('builds the span on the venue clock', () => {
    const info = buildBookingInfo(slotInfo, '+541122334455');

    expect(info.startsAt).toBe('2026-03-20T13:00:00.000Z');
    expect(info.endsAt).toBe('2026-03-20T14:30:00.000Z');
  });

  // A slot that runs past midnight comes back with an end smaller than its
  // start. Compared as numbers that span runs backwards; pushed onto the next
  // day it is two hours, which is what the customer picked.
  it('carries a slot that runs past midnight onto the next day', () => {
    const info = buildBookingInfo(
      { ...slotInfo, startTime: '23:00', endTime: '01:00', durationMinutes: 120 },
      '+541122334455',
    );

    expect(info.startsAt).toBe('2026-03-21T02:00:00.000Z');
    expect(info.endsAt).toBe('2026-03-21T04:00:00.000Z');
  });

  it('computes depositAmount from the deposit percentage', () => {
    const info = buildBookingInfo(slotInfo, '+541122334455');
    expect(info.depositAmount).toBe(300_000);
  });

  it('falls back to full price as depositAmount when depositPercentage is 0', () => {
    const info = buildBookingInfo({ ...slotInfo, depositPercentage: 0 }, '+541122334455');
    expect(info.depositAmount).toBe(1_000_000);
  });

  it('maps the remaining BookingInfo fields from slotInfo', () => {
    const info = buildBookingInfo(slotInfo, '+541122334455');
    expect(info).toMatchObject({
      courtName: 'Cancha 1',
      date: '2026-03-20',
      startTime: '10:00',
      startsAt: '2026-03-20T13:00:00.000Z',
      endsAt: '2026-03-20T14:30:00.000Z',
      price: 1_000_000,
      complexName: 'Club Norte',
      complexPhone: '1155550000',
      cancellationHours: 24,
      clientPhone: '+541122334455',
    });
  });
});
