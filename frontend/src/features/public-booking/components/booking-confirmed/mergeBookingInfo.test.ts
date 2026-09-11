// @vitest-environment node
import { mergeBookingInfo } from './mergeBookingInfo';
import type { BookingInfo } from './types';
import type { BookingStatusDetails } from '@/shared/types/api.types';

const cached: BookingInfo = {
  courtName: 'Cancha 1',
  date: '2026-03-18',
  startTime: '23:00',
  startsAt: '2026-03-18T23:00:00-03:00',
  endsAt: '2026-03-19T01:00:00-03:00',
  price: 1_000_000,
  depositAmount: 300_000,
  complexName: 'Club Norte',
  complexPhone: '1155550000',
  cancellationHours: 24,
  clientPhone: '1122334455',
};

const details: BookingStatusDetails = {
  status: 'confirmed',
  collection_status: 'deposit_paid',
  refund_status: 'none',
  court_name: 'Cancha API',
  complex_name: 'Club API',
  date: '2026-03-18',
  start_time: '23:00',
  starts_at: '2026-03-18T23:00:00-03:00',
  ends_at: '2026-03-19T01:00:00-03:00',
  price: 2_000_000,
  deposit_amount: 600_000,
};

describe('mergeBookingInfo', () => {
  it('takes the end from the live answer when it has one', () => {
    const merged = mergeBookingInfo({ ...details, ends_at: '2026-03-19T02:00:00-03:00' }, cached);

    expect(merged?.endsAt).toBe('2026-03-19T02:00:00-03:00');
  });

  it('falls back to the cached end when the answer has not caught up', () => {
    const { ends_at: _dropped, ...withoutEnd } = details;

    expect(mergeBookingInfo(withoutEnd, cached)?.endsAt).toBe(cached.endsAt);
  });

  // The gate, and it is the reason this file exists. It used to refuse on a
  // missing `end_time`; the field is gone (backend dropped the column) and the
  // refusal has to move with it rather than quietly disappear. A merge that
  // returned a BookingInfo with no end would put a card on the confirmation
  // screen with the hours half-missing, in front of somebody who has just paid.
  it('refuses to build a card with no end at all', () => {
    const { ends_at: _dropped, ...withoutEnd } = details;
    const { endsAt: _alsoDropped, ...cachedWithoutEnd } = cached;

    expect(mergeBookingInfo(withoutEnd, cachedWithoutEnd as BookingInfo)).toBeNull();
  });

  it('refuses on a missing start instant too', () => {
    const { starts_at: _dropped, ...withoutStart } = details;
    const { startsAt: _alsoDropped, ...cachedWithoutStart } = cached;

    expect(mergeBookingInfo(withoutStart, cachedWithoutStart as BookingInfo)).toBeNull();
  });

  // The control: everything else present and only the fields under test
  // supplied, so the two refusals above are about the instants and not about
  // the merge refusing on general principle.
  it('builds a card when both instants are there', () => {
    const merged = mergeBookingInfo(details, null);

    expect(merged).not.toBeNull();
    expect(merged?.startsAt).toBe('2026-03-18T23:00:00-03:00');
    expect(merged?.endsAt).toBe('2026-03-19T01:00:00-03:00');
  });
});
