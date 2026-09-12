import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { useBookingStatus, type BookingStatusView } from '@/features/public-booking';

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));
vi.mock('@/features/public-booking/hooks/useBookingStatus', () => ({
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
vi.mock('@/features/public-booking/components/BookingConfirmed', () => ({
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

async function renderPage(path: string, state: unknown = null) {
  const Page = (await import('./BookSuccessPage')).default;
  const [pathname = '', search] = path.split('?');
  return render(
    <MemoryRouter initialEntries={[{ pathname, search: search ? `?${search}` : '', state }]}>
      <Routes>
        <Route path="/:slug/book/success" element={<Page />} />
        <Route path="/:slug" element={<div>slot selection page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  lastProps = undefined;
  sessionStorage.clear();
});

describe('BookSuccessPage token resolution', () => {
  it('reads the token from location.state when present (no-deposit flow)', async () => {
    await renderPage('/club-norte/book/success', { token: 't1' });
    expect(screen.getByText('token:t1')).toBeInTheDocument();
  });

  it('falls back to the token search param when no state is present (MP redirect flow)', async () => {
    await renderPage('/club-norte/book/success?token=t2');
    expect(screen.getByText('token:t2')).toBeInTheDocument();
  });

  it('resolves to null when neither state nor search param is present', async () => {
    await renderPage('/club-norte/book/success');
    expect(screen.getByText('token:null')).toBeInTheDocument();
  });
});

// A full, schema-valid `BookingInfo` shape. bookingInfoSchema.safeParse now
// validates `location.state.bookingInfo` and the sessionStorage fallback, so
// a fixture missing any required field is rejected rather than passed through.
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
  it('reads bookingInfo from location.state when present', async () => {
    await renderPage('/club-norte/book/success', { token: 't1', bookingInfo: validBookingInfo });
    expect(lastProps?.bookingInfo).toEqual(validBookingInfo);
  });

  it('falls back to sessionStorage when no state bookingInfo is present (MP redirect flow)', async () => {
    const stored = { ...validBookingInfo, courtName: 'Cancha 2' };
    sessionStorage.setItem('vibe_booking_info', JSON.stringify(stored));
    await renderPage('/club-norte/book/success?token=t2');
    expect(lastProps?.bookingInfo).toEqual(stored);
  });

  it('falls back to null when location.state.bookingInfo is shaped wrong (missing required fields)', async () => {
    await renderPage('/club-norte/book/success', {
      token: 't1',
      bookingInfo: { courtName: 'Cancha 1' },
    });
    expect(lastProps?.bookingInfo).toBeNull();
  });

  it('falls back to null when the sessionStorage entry is shaped wrong', async () => {
    sessionStorage.setItem('vibe_booking_info', JSON.stringify({ courtName: 'Cancha 2' }));
    await renderPage('/club-norte/book/success?token=t2');
    expect(lastProps?.bookingInfo).toBeNull();
  });

  it('falls back to null when a field in the stored booking info is null instead of the expected type', async () => {
    sessionStorage.setItem('vibe_booking_info', JSON.stringify({ ...validBookingInfo, price: null }));
    await renderPage('/club-norte/book/success?token=t2');
    expect(lastProps?.bookingInfo).toBeNull();
  });
});

describe('BookSuccessPage location.state validation', () => {
  it('ignores a garbage location.state and still resolves the token from the search param', async () => {
    await renderPage('/club-norte/book/success?token=t2', 'not-an-object');
    expect(screen.getByText('token:t2')).toBeInTheDocument();
  });

  it('ignores a location.state with a token of the wrong type', async () => {
    await renderPage('/club-norte/book/success?token=t2', { token: 12345 });
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
    await renderPage('/club-norte/book/success', { token: 't1' });
    expect(lastProps?.bookingDetails).toEqual(details);
  });
});

describe('BookSuccessPage payment-under-review resolution', () => {
  it('reads status=pending from the MP back_url and resolves the under_review view', async () => {
    await renderPage('/club-norte/book/success?token=t2&status=pending');
    expect(screen.getByText('view:under_review')).toBeInTheDocument();
  });

  it('resolves the loading view when status is absent from the URL and nothing has answered yet', async () => {
    await renderPage('/club-norte/book/success?token=t2');
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
    await renderPage('/club-norte/book/success', { token: 't1' });
    expect(screen.getByText('view:error')).toBeInTheDocument();
  });

  it('refetches the status query on status-retry, without navigating away', async () => {
    const user = userEvent.setup();
    const refetch = vi.fn();
    vi.mocked(useBookingStatus).mockReturnValueOnce({
      data: undefined,
      isLoading: false,
      isError: true,
      timedOut: false,
      refetch,
    } as unknown as ReturnType<typeof useBookingStatus>);
    await renderPage('/club-norte/book/success', { token: 't1' });
    await user.click(screen.getByText('status-retry'));
    expect(refetch).toHaveBeenCalledTimes(1);
    expect(screen.queryByText('slot selection page')).not.toBeInTheDocument();
  });
});

describe('BookSuccessPage retry navigation', () => {
  it('navigates back to the complex slug page on retry', async () => {
    const user = userEvent.setup();
    await renderPage('/club-norte/book/success', { token: 't1' });
    await user.click(screen.getByText('retry'));
    expect(screen.getByText('slot selection page')).toBeInTheDocument();
  });
});
