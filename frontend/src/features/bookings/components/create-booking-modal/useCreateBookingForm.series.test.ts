import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { toast } from 'sonner';
import { makeConsumedHttpError, makeCourt, makePrice } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useCreateBookingForm } from './useCreateBookingForm';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

vi.mock('../../api/bookingSeries.api', () => ({ bookingSeriesApi: { create: vi.fn() } }));
vi.mock('../../api/bookings.api', () => ({ bookingsApi: { create: vi.fn(), list: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), warning: vi.fn() } }));

const days = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'] as const;
const SCHEDULES: Schedule[] = days.map((day, i) => ({
  id: `s${String(i)}`,
  complex_id: 'c1',
  day,
  open_time: '08:00',
  close_time: '23:00',
  is_closed: false,
}));

// 2099-01-02 is a Friday, far enough ahead to clear the "not in the past" rule.
const FRIDAY = '2099-01-02';

const court: CourtWithPrices = {
  ...makeCourt({ id: 'ct1' }),
  prices: [makePrice({ day_type: 'friday', time_from: '08:00', time_to: '23:00', price: 100000 })],
};

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return createElement(QueryClientProvider, { client: queryClient }, children);
}

const SERIES_BODY = {
  court_id: 'ct1',
  first_date: FRIDAY,
  start_time: '10:00',
  duration_minutes: 60,
  client_phone: '+541155550000',
  client_first_name: 'Juan',
  client_last_name: 'Perez',
};

// 2099-01-02 is a Friday; the series holds its dates, so no bookings come back.
const CREATED = { series: { weekday: 5, start_time: '10:00:00' }, bookings: [], skipped: [{}] };

/** The form with a filled-in booking and the repeat switch on. */
async function renderRepeatingForm() {
  const hook = renderHook(
    () =>
      useCreateBookingForm({
        open: false,
        complexId: 'c1',
        courts: [court],
        depositPercentage: 0,
        schedules: SCHEDULES,
      }),
    { wrapper },
  );
  act(() => {
    const form = hook.result.current;
    form.setValue('court_id', 'ct1');
    form.setValue('date', FRIDAY);
    form.setValue('start_time', '10:00');
    form.setValue('duration_minutes', 60);
    form.setValue('client_phone', '+541155550000');
    form.setValue('client_first_name', 'Juan');
    form.setValue('client_last_name', 'Perez');
    form.series.setRepeat(true);
  });
  await waitFor(() => {
    expect(hook.result.current.priceRequired).toBe(false);
  });
  return hook.result;
}

async function submit(result: Awaited<ReturnType<typeof renderRepeatingForm>>, onClose = vi.fn()) {
  await act(async () => {
    await result.current.handleSubmit((data) => {
      result.current.onSubmit(data, onClose);
    })();
  });
  return onClose;
}

describe('useCreateBookingForm — repeat every week', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('with no end, calls the series endpoint (not the single-booking one) with no ends_on or weeks', async () => {
    const { bookingSeriesApi } = await import('../../api/bookingSeries.api');
    const { bookingsApi } = await import('../../api/bookings.api');
    vi.mocked(bookingSeriesApi.create).mockResolvedValueOnce(CREATED as never);
    const result = await renderRepeatingForm();

    const onClose = await submit(result);

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
    expect(bookingSeriesApi.create).toHaveBeenCalledWith('c1', SERIES_BODY, expect.any(String));
    expect(bookingsApi.create).not.toHaveBeenCalled();
    expect(toast.success).toHaveBeenCalledWith(ES_AR.bookings.series.createSuccess('viernes', '10:00', 1));
  });
});

describe('useCreateBookingForm — repeat end choice', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('"Hasta" sends ends_on', async () => {
    const { bookingSeriesApi } = await import('../../api/bookingSeries.api');
    vi.mocked(bookingSeriesApi.create).mockResolvedValueOnce(CREATED as never);
    const result = await renderRepeatingForm();
    act(() => {
      result.current.series.setEndMode('until');
      result.current.series.setUntil('2099-03-27');
    });

    await submit(result);

    expect(bookingSeriesApi.create).toHaveBeenCalledWith(
      'c1',
      { ...SERIES_BODY, ends_on: '2099-03-27' },
      expect.any(String),
    );
  });

  it('"Por N semanas" sends weeks, never ends_on', async () => {
    const { bookingSeriesApi } = await import('../../api/bookingSeries.api');
    vi.mocked(bookingSeriesApi.create).mockResolvedValueOnce(CREATED as never);
    const result = await renderRepeatingForm();
    act(() => {
      result.current.series.setEndMode('weeks');
      result.current.series.setWeeks('12');
    });

    await submit(result);

    expect(bookingSeriesApi.create).toHaveBeenCalledWith('c1', { ...SERIES_BODY, weeks: 12 }, expect.any(String));
  });

  it('drops a deposit chosen before the switch, so the series carries no payment fields', async () => {
    const { bookingSeriesApi } = await import('../../api/bookingSeries.api');
    vi.mocked(bookingSeriesApi.create).mockResolvedValueOnce(CREATED as never);
    const result = await renderRepeatingForm();
    act(() => {
      result.current.series.setRepeat(false);
      result.current.setValue('payment_option', 'deposit');
      result.current.series.setRepeat(true);
    });

    await submit(result);

    expect(bookingSeriesApi.create).toHaveBeenCalledWith('c1', SERIES_BODY, expect.any(String));
  });

  it('rejects a weeks count outside 2-52 before it reaches the server', async () => {
    const result = await renderRepeatingForm();
    act(() => {
      result.current.series.setEndMode('weeks');
      result.current.series.setWeeks('1');
    });

    let valid = true;
    act(() => {
      valid = result.current.series.validate(FRIDAY);
    });

    expect(valid).toBe(false);
    expect(result.current.series.error).toBe(ES_AR.bookings.series.weeksRange);
  });
});

describe('useCreateBookingForm — repeat conflicts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const CONFLICTS = [
    { date: '2099-01-09', reason: 'overlap' },
    { date: '2099-01-16', reason: 'closed_day' },
  ];

  it('keeps the refused dates for the modal instead of toasting, and resubmits with skip_conflicts', async () => {
    const { bookingSeriesApi } = await import('../../api/bookingSeries.api');
    vi.mocked(bookingSeriesApi.create)
      .mockRejectedValueOnce(await makeConsumedHttpError(409, { title: 'Conflict', conflicts: CONFLICTS }))
      .mockResolvedValueOnce(CREATED as never);
    const result = await renderRepeatingForm();

    const onClose = await submit(result);

    await waitFor(() => {
      expect(result.current.seriesSubmit.conflicts).toEqual(CONFLICTS);
    });
    expect(toast.error).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();

    act(() => {
      result.current.seriesSubmit.submitSkippingConflicts(onClose);
    });

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
    expect(bookingSeriesApi.create).toHaveBeenLastCalledWith(
      'c1',
      { ...SERIES_BODY, skip_conflicts: true },
      expect.any(String),
    );
  });

  it('toasts the server message for a 409 that names no date', async () => {
    const { bookingSeriesApi } = await import('../../api/bookingSeries.api');
    vi.mocked(bookingSeriesApi.create).mockRejectedValueOnce(
      await makeConsumedHttpError(409, { title: 'Conflict', detail: 'El cliente está bloqueado' }),
    );
    const result = await renderRepeatingForm();

    await submit(result);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('El cliente está bloqueado');
    });
    expect(result.current.seriesSubmit.conflicts).toEqual([]);
  });

  it('"Volver" clears the list so the form is back', async () => {
    const { bookingSeriesApi } = await import('../../api/bookingSeries.api');
    vi.mocked(bookingSeriesApi.create).mockRejectedValueOnce(
      await makeConsumedHttpError(409, { title: 'Conflict', conflicts: CONFLICTS }),
    );
    const result = await renderRepeatingForm();
    await submit(result);
    await waitFor(() => {
      expect(result.current.seriesSubmit.conflicts).toHaveLength(2);
    });

    act(() => {
      result.current.seriesSubmit.dismissConflicts();
    });

    expect(result.current.seriesSubmit.conflicts).toEqual([]);
  });
});
