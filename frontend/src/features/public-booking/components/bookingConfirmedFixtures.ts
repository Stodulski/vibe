/**
 * Shared fixtures for `BookingConfirmed.test.tsx` and
 * `BookingConfirmed.bookingDetails.test.tsx` — split across two files only
 * because a single one would cross the repo's 300-line-per-file lint cap,
 * not because the two suites are testing different things.
 */
export const mockBookingInfo = {
  courtName: 'Cancha 1',
  date: '2026-03-16',
  startTime: '10:00',
  // The span, as the venue's clock reads it. The pair used to be two HH:MM
  // strings; the end is an instant now because it is the only field that can
  // say a booking finishes on the following day.
  startsAt: '2026-03-16T10:00:00-03:00',
  endsAt: '2026-03-16T11:30:00-03:00',
  price: 1500000,
  depositAmount: 500000,
  complexName: 'Club Test',
  complexPhone: '+5491155550000',
  cancellationHours: 24,
  clientPhone: '+5491155551111',
};

// `onRetry`/`onStatusRetry` are deliberately left out — they must be fresh
// `vi.fn()`s owned by whichever test file spreads this in, and `vi` isn't
// typed for a file outside the `*.test.ts(x)` glob (see tsconfig.test.json).
export const baseProps = {
  view: { kind: 'confirmed' } as const,
  status: 'confirmed' as const,
  collectionStatus: 'deposit_paid' as const,
  bookingInfo: mockBookingInfo,
  token: 'tok-abc12345-6789-0000-0000-000000000000',
  slug: 'club-test',
};
