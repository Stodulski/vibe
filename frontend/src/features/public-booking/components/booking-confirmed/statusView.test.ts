import { resolveBookingStatusView, type BookingStatusSignals } from './statusView';

const base: BookingStatusSignals = {
  status: undefined,
  collectionStatus: undefined,
  isLoading: false,
  isError: false,
  timedOut: false,
  paymentUnderReview: false,
  linkExpired: false,
  linkNotFound: false,
};

describe('resolveBookingStatusView', () => {
  it('resolves to loading while the first fetch is in flight', () => {
    expect(resolveBookingStatusView({ ...base, isLoading: true })).toEqual({ kind: 'loading' });
  });

  it('resolves to loading before any data has arrived, even if isLoading already flipped off', () => {
    expect(resolveBookingStatusView({ ...base })).toEqual({ kind: 'loading' });
  });

  // M2: the original bug. A network drop or a 500 (not 404/410) left status
  // and collectionStatus both undefined, which used to read as "loading"
  // forever and then, after 120s, as "still processing" — never as an error.
  it('resolves to error, not loading, when the query failed and there is no data at all', () => {
    expect(resolveBookingStatusView({ ...base, isError: true })).toEqual({ kind: 'error' });
  });

  it('resolves to error even after the 120s timeout has fired, ahead of the misleading "still processing" copy', () => {
    expect(resolveBookingStatusView({ ...base, isError: true, timedOut: true })).toEqual({
      kind: 'error',
    });
  });

  it('does not resolve to error when a background poll fails after a prior answer already arrived', () => {
    const signals: BookingStatusSignals = {
      ...base,
      status: 'pending',
      collectionStatus: 'unpaid',
      isError: true,
    };
    expect(resolveBookingStatusView(signals)).toEqual({ kind: 'pending' });
  });

  it('resolves to timed_out once the 120s poll timeout fires with no terminal status yet', () => {
    const signals: BookingStatusSignals = {
      ...base,
      status: 'pending',
      collectionStatus: 'unpaid',
      timedOut: true,
    };
    expect(resolveBookingStatusView(signals)).toEqual({ kind: 'timed_out' });
  });

  it('resolves to pending while collection_status is unpaid', () => {
    const signals: BookingStatusSignals = {
      ...base,
      status: 'pending',
      collectionStatus: 'unpaid',
    };
    expect(resolveBookingStatusView(signals)).toEqual({ kind: 'pending' });
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call.
describe('resolveBookingStatusView — terminal and priority signals', () => {
  it('resolves to cancelled', () => {
    const signals: BookingStatusSignals = {
      ...base,
      status: 'cancelled',
      collectionStatus: 'unpaid',
    };
    expect(resolveBookingStatusView(signals)).toEqual({ kind: 'cancelled' });
  });

  it('resolves to confirmed for a terminal, non-cancelled status', () => {
    const signals: BookingStatusSignals = {
      ...base,
      status: 'confirmed',
      collectionStatus: 'deposit_paid',
    };
    expect(resolveBookingStatusView(signals)).toEqual({ kind: 'confirmed' });
  });

  it('resolves to link_expired ahead of every other signal', () => {
    expect(resolveBookingStatusView({ ...base, linkExpired: true, isLoading: true })).toEqual({
      kind: 'link_expired',
    });
  });

  it('resolves to link_not_found, not link_expired, on a 404', () => {
    expect(resolveBookingStatusView({ ...base, linkNotFound: true })).toEqual({
      kind: 'link_not_found',
    });
  });

  it('resolves to under_review ahead of the loading state', () => {
    expect(resolveBookingStatusView({ ...base, isLoading: true, paymentUnderReview: true })).toEqual({
      kind: 'under_review',
    });
  });

  it('resolves to under_review ahead of a genuine fetch error', () => {
    expect(resolveBookingStatusView({ ...base, isError: true, paymentUnderReview: true })).toEqual({
      kind: 'under_review',
    });
  });

  it('steps aside from under_review once the booking reaches a terminal status', () => {
    const signals: BookingStatusSignals = {
      ...base,
      status: 'confirmed',
      collectionStatus: 'deposit_paid',
      paymentUnderReview: true,
    };
    expect(resolveBookingStatusView(signals)).toEqual({ kind: 'confirmed' });
  });
});
