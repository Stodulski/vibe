import { describe, it, expect, vi, beforeEach, afterEach, type MockInstance } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useBookingStatus } from '../hooks/useBookingStatus';
import type { BookingStatusView } from '../components/booking-confirmed/statusView';
import { setBookingUrl } from '../test/booking';
import { bookingResultKey } from '../lib/handoff';

vi.mock('../hooks/useBookingStatus', () => ({
  useBookingStatus: vi.fn().mockReturnValue({
    data: undefined,
    isLoading: true,
    isError: false,
    timedOut: false,
    refetch: vi.fn(),
  }),
}));

interface CapturedProps {
  token: string | null;
  bookingInfo: unknown;
  bookingDetails: unknown;
  view: BookingStatusView;
  onRetry: () => void;
  onStatusRetry: () => void;
}

let lastProps: CapturedProps | undefined;
vi.mock('../components/BookingConfirmed', () => ({
  BookingConfirmed: (props: CapturedProps) => {
    lastProps = props;
    return (
      <div data-testid="booking-confirmed">
        <span>token:{props.token ?? 'null'}</span>
        <span>view:{props.view.kind}</span>
        <button onClick={props.onRetry}>retry</button>
        <button onClick={props.onStatusRetry}>status-retry</button>
      </div>
    );
  },
}));

/**
 * Renders the success page at `path`. The confirm page leaves its result for this
 * slug in sessionStorage, which is what the page reads on mount. A test can park a
 * different value there, raw, to check that the schema rejects it.
 */
async function renderPage(path: string, result: unknown = null) {
  const Page = (await import('./BookSuccessPage')).default;
  setBookingUrl(path);
  if (result !== null) {
    window.sessionStorage.setItem(bookingResultKey('club-norte'), JSON.stringify(result));
  }
  return render(<Page slug="club-norte" />);
}

let replaceSpy: MockInstance<Location['replace']> | undefined;

beforeEach(() => {
  lastProps = undefined;
  sessionStorage.clear();
});

afterEach(() => {
  replaceSpy?.mockRestore();
  replaceSpy = undefined;
  setBookingUrl('/');
});

describe('BookSuccessPage token resolution', () => {
  it('reads the token from the saved result when present (no-deposit flow)', async () => {
    await renderPage('/c/club-norte/book/success', { token: 't1' });
    expect(screen.getByText('token:t1')).toBeInTheDocument();
  });

  it('falls back to the token search param when no result is saved (MP redirect flow)', async () => {
    await renderPage('/c/club-norte/book/success?token=t2');
    expect(screen.getByText('token:t2')).toBeInTheDocument();
  });

  it('resolves to null when neither the result nor the search param is present', async () => {
    await renderPage('/c/club-norte/book/success');
    expect(screen.getByText('token:null')).toBeInTheDocument();
  });
});

// A full, schema-valid `BookingInfo` shape. bookingInfoSchema validates the saved
// result's bookingInfo and the sessionStorage fallback, so a fixture missing any
// required field is rejected rather than passed through.
const validBookingInfo = {
  courtName: 'Cancha 1',
  date: '2026-03-20',
  startTime: '10:00',
  startsAt: '2026-03-20T10:00:00-03:00',
  endsAt: '2026-03-20T11:30:00-03:00',
  price: 1_000_000,
  depositAmount: 300_000,
  complexName: 'Club Norte',
  complexPhone: '1155550000',
  cancellationHours: 24,
  clientPhone: '1122334455',
};

describe('BookSuccessPage bookingInfo resolution', () => {
  it('reads bookingInfo from the saved result when present', async () => {
    await renderPage('/c/club-norte/book/success', { token: 't1', bookingInfo: validBookingInfo });
    expect(lastProps?.bookingInfo).toEqual(validBookingInfo);
  });

  it('falls back to sessionStorage when the result has no bookingInfo (MP redirect flow)', async () => {
    const stored = { ...validBookingInfo, courtName: 'Cancha 2' };
    sessionStorage.setItem('vibe_booking_info', JSON.stringify(stored));
    await renderPage('/c/club-norte/book/success?token=t2');
    expect(lastProps?.bookingInfo).toEqual(stored);
  });

  it('falls back to null when the saved result has a bookingInfo shaped wrong (missing required fields)', async () => {
    await renderPage('/c/club-norte/book/success', {
      token: 't1',
      bookingInfo: { courtName: 'Cancha 1' },
    });
    expect(lastProps?.bookingInfo).toBeNull();
  });

  it('falls back to null when the sessionStorage entry is shaped wrong', async () => {
    sessionStorage.setItem('vibe_booking_info', JSON.stringify({ courtName: 'Cancha 2' }));
    await renderPage('/c/club-norte/book/success?token=t2');
    expect(lastProps?.bookingInfo).toBeNull();
  });

  it('falls back to null when a field in the stored booking info is null instead of the expected type', async () => {
    sessionStorage.setItem('vibe_booking_info', JSON.stringify({ ...validBookingInfo, price: null }));
    await renderPage('/c/club-norte/book/success?token=t2');
    expect(lastProps?.bookingInfo).toBeNull();
  });
});

describe('BookSuccessPage saved result validation', () => {
  it('ignores a garbage saved result and still resolves the token from the search param', async () => {
    await renderPage('/c/club-norte/book/success?token=t2', 'not-an-object');
    expect(screen.getByText('token:t2')).toBeInTheDocument();
  });

  it('ignores a saved result with a token of the wrong type', async () => {
    await renderPage('/c/club-norte/book/success?token=t2', { token: 12345 });
    expect(screen.getByText('token:t2')).toBeInTheDocument();
  });
});

describe('BookSuccessPage bookingDetails resolution', () => {
  it("forwards useBookingStatus's data straight through as bookingDetails", async () => {
    const details = {
      status: 'confirmed',
      collection_status: 'deposit_paid',
      refund_status: 'none',
      complex_name: 'Club API',
    };
    vi.mocked(useBookingStatus).mockReturnValueOnce({
      data: details,
      isLoading: false,
      isError: false,
      timedOut: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useBookingStatus>);
    await renderPage('/c/club-norte/book/success', { token: 't1' });
    expect(lastProps?.bookingDetails).toEqual(details);
  });
});

describe('BookSuccessPage payment-under-review resolution', () => {
  it('reads status=pending from the MP back_url and resolves the under_review view', async () => {
    await renderPage('/c/club-norte/book/success?token=t2&status=pending');
    expect(screen.getByText('view:under_review')).toBeInTheDocument();
  });

  it('resolves the loading view when status is absent from the URL and nothing has answered yet', async () => {
    await renderPage('/c/club-norte/book/success?token=t2');
    expect(screen.getByText('view:loading')).toBeInTheDocument();
  });
});

// M2/M11: `GET /book/status` failing outright (a dropped connection or a
// 500, not the 404/410 `resolveLink` cases already covered elsewhere) used
// to be indistinguishable from "still loading" — this is the regression test
// for the fix.
describe('BookSuccessPage status error resolution', () => {
  it('resolves the error view, not loading, when the status request fails with no prior data', async () => {
    vi.mocked(useBookingStatus).mockReturnValueOnce({
      data: undefined,
      isLoading: false,
      isError: true,
      timedOut: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useBookingStatus>);
    await renderPage('/c/club-norte/book/success', { token: 't1' });
    expect(screen.getByText('view:error')).toBeInTheDocument();
  });

  it('refetches the status query on status-retry, without leaving the page', async () => {
    const user = userEvent.setup();
    const refetch = vi.fn();
    replaceSpy = vi.spyOn(window.location, 'replace').mockImplementation(() => undefined);
    vi.mocked(useBookingStatus).mockReturnValueOnce({
      data: undefined,
      isLoading: false,
      isError: true,
      timedOut: false,
      refetch,
    } as unknown as ReturnType<typeof useBookingStatus>);
    await renderPage('/c/club-norte/book/success', { token: 't1' });
    await user.click(screen.getByText('status-retry'));
    expect(refetch).toHaveBeenCalledTimes(1);
    expect(replaceSpy).not.toHaveBeenCalled();
  });
});

describe('BookSuccessPage retry navigation', () => {
  it('navigates back to the complex slug page on retry', async () => {
    const user = userEvent.setup();
    replaceSpy = vi.spyOn(window.location, 'replace').mockImplementation(() => undefined);
    await renderPage('/c/club-norte/book/success', { token: 't1' });
    await user.click(screen.getByText('retry'));
    expect(replaceSpy).toHaveBeenCalledWith('/c/club-norte');
  });
});
