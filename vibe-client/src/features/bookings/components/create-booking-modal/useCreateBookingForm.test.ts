import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { makeCourt, makePrice } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useCreateBookingForm } from './useCreateBookingForm';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

const t = ES_AR;

vi.mock('../../api/bookings.api', () => ({
  bookingsApi: { create: vi.fn(), list: vi.fn() },
}));

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn(), warning: vi.fn() },
}));

function openEveryDay(open: string, close: string): Schedule[] {
  const days = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'] as const;
  return days.map((day, i) => ({
    id: `s${String(i)}`,
    complex_id: 'c1',
    day,
    open_time: open,
    close_time: close,
    is_closed: false,
  }));
}

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return createElement(QueryClientProvider, { client: queryClient }, children);
}

// 2099-01-02 is a Friday, far enough out to stay ahead of the schema's
// "date/time can't be in the past" refine regardless of when this test runs.
const FRIDAY = '2099-01-02';
const SCHEDULES = openEveryDay('08:00', '23:00');

/** A court with no price bands at all: `findTotalPrice` is always `null`. */
function unpricedCourt(): CourtWithPrices {
  return { ...makeCourt({ id: 'ct1' }), prices: [] };
}

// `price` here is already centavos (matches `pricing.test.ts`'s fixtures),
// so a 60-minute booking (two 30-minute blocks) prices at 100000 centavos =
// $1000 — a 20% deposit is a round 200 pesos (20000 centavos).
function pricedCourt(): CourtWithPrices {
  return {
    ...makeCourt({ id: 'ct1' }),
    prices: [makePrice({ day_type: 'friday', time_from: '08:00', time_to: '23:00', price: 100000 })],
  };
}

function fillClientAndSchedule(form: ReturnType<typeof useCreateBookingForm>, courtId: string) {
  form.setValue('court_id', courtId);
  form.setValue('date', FRIDAY);
  form.setValue('start_time', '10:00');
  form.setValue('duration_minutes', 60);
  form.setValue('client_phone', '+541155550000');
  form.setValue('client_first_name', 'Juan');
  form.setValue('client_last_name', 'Perez');
}

describe('useCreateBookingForm — manual price required (no schema-side effect, 02-bookings-clients.md M3)', () => {
  it('blocks submit and surfaces a field error when no price rule covers the span and no manual price was entered', async () => {
    const { bookingsApi } = await import('../../api/bookings.api');
    const { result } = renderHook(
      () =>
        useCreateBookingForm({
          open: false,
          complexId: 'c1',
          courts: [unpricedCourt()],
          depositPercentage: 0,
          schedules: SCHEDULES,
        }),
      { wrapper },
    );

    act(() => {
      fillClientAndSchedule(result.current, 'ct1');
    });
    await waitFor(() => {
      expect(result.current.priceRequired).toBe(true);
    });

    await act(async () => {
      await result.current.handleSubmit((data) => {
        result.current.onSubmit(data, vi.fn());
      })();
    });

    expect(bookingsApi.create).not.toHaveBeenCalled();
    expect(result.current.errors.price?.message).toBe(t.validation.manualPriceRequired);
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body (the header line's `() => {`
// included), so this stays a genuinely separate top-level call.
describe('useCreateBookingForm — manual price entered (02-bookings-clients.md M3)', () => {
  it('submits the manual price (in cents) once one is entered', async () => {
    const { bookingsApi } = await import('../../api/bookings.api');
    vi.mocked(bookingsApi.create).mockResolvedValueOnce({ booking: {} } as never);
    const onClose = vi.fn();
    const { result } = renderHook(
      () =>
        useCreateBookingForm({
          open: false,
          complexId: 'c1',
          courts: [unpricedCourt()],
          depositPercentage: 0,
          schedules: SCHEDULES,
        }),
      { wrapper },
    );

    act(() => {
      fillClientAndSchedule(result.current, 'ct1');
      result.current.setValue('price', 5000);
    });
    await waitFor(() => {
      expect(result.current.priceRequired).toBe(true);
    });

    await act(async () => {
      await result.current.handleSubmit((data) => {
        result.current.onSubmit(data, onClose);
      })();
    });

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
    expect(bookingsApi.create).toHaveBeenCalledWith('c1', expect.objectContaining({ price: 500000 }));
  });
});

describe('useCreateBookingForm — deposit amount follows the current price at submit (02-bookings-clients.md M3)', () => {
  it('applies the fresh price-derived default when the field was never touched by hand', async () => {
    const { bookingsApi } = await import('../../api/bookings.api');
    vi.mocked(bookingsApi.create).mockResolvedValueOnce({ booking: {} } as never);
    const onClose = vi.fn();
    const { result } = renderHook(
      () =>
        useCreateBookingForm({
          open: false,
          complexId: 'c1',
          courts: [pricedCourt()],
          depositPercentage: 20,
          schedules: SCHEDULES,
        }),
      { wrapper },
    );

    act(() => {
      fillClientAndSchedule(result.current, 'ct1');
      // Selecting "deposit" without going through `PaymentOptionFields`'s own
      // onChange (which would have set the default itself) — nothing ever
      // writes `deposit_amount`, so it stays undefined and un-dirtied.
      result.current.setValue('payment_option', 'deposit');
      result.current.setValue('payment_method', 'cash');
    });
    await waitFor(() => {
      expect(result.current.defaultDepositPesos).toBe(200);
    });

    await act(async () => {
      await result.current.handleSubmit((data) => {
        result.current.onSubmit(data, onClose);
      })();
    });

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
    expect(bookingsApi.create).toHaveBeenCalledWith('c1', expect.objectContaining({ deposit_amount: 20000 }));
  });
});

// A sibling describe, not nested in the one above — same reasoning as above.
describe('useCreateBookingForm — deposit amount kept when hand-entered (02-bookings-clients.md M3)', () => {
  it('keeps a manually-entered deposit amount instead of overwriting it with the default', async () => {
    const { bookingsApi } = await import('../../api/bookings.api');
    vi.mocked(bookingsApi.create).mockResolvedValueOnce({ booking: {} } as never);
    const onClose = vi.fn();
    const { result } = renderHook(
      () =>
        useCreateBookingForm({
          open: false,
          complexId: 'c1',
          courts: [pricedCourt()],
          depositPercentage: 20,
          schedules: SCHEDULES,
        }),
      { wrapper },
    );

    act(() => {
      fillClientAndSchedule(result.current, 'ct1');
      result.current.setValue('payment_option', 'deposit');
      result.current.setValue('payment_method', 'cash');
      // The `{ shouldDirty: true }` here is what `DepositAmountField`'s own
      // onChange passes for a real keystroke.
      result.current.setValue('deposit_amount', 3500, { shouldDirty: true });
    });
    await waitFor(() => {
      expect(result.current.defaultDepositPesos).toBe(200);
    });

    await act(async () => {
      await result.current.handleSubmit((data) => {
        result.current.onSubmit(data, onClose);
      })();
    });

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
    expect(bookingsApi.create).toHaveBeenCalledWith('c1', expect.objectContaining({ deposit_amount: 350000 }));
  });
});
