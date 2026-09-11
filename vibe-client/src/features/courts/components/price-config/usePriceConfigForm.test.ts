import { createElement, type ReactNode } from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { toast } from 'sonner';
import { makeConsumedHttpError, makeComplex } from '@/test/factories';
import { courtsApi } from '../../api/courts.api';
import { usePriceConfigForm } from './usePriceConfigForm';
import { EMPTY_PRICE_FORM_VALUES } from './days';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

vi.mock('../../api/courts.api', () => ({
  courtsApi: { updatePrices: vi.fn() },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

let mockSchedules: Schedule[] = [];

vi.mock('@/features/complex/hooks/useComplex', () => ({
  useComplex: () => ({ data: makeComplex({ id: 'c1', slug: 'los-alamos' }) }),
}));

vi.mock('@/features/complex/hooks/useSchedules', () => ({
  useSchedules: () => ({ data: mockSchedules }),
}));

function schedule(overrides: Partial<Schedule>): Schedule {
  return {
    id: 's1',
    complex_id: 'c1',
    day: 'monday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
    ...overrides,
  };
}

const court: CourtWithPrices = {
  id: 'ct1',
  complex_id: 'c1',
  name: 'Cancha 1',
  sport: 'padel',
  court_type: 'outdoor',
  is_active: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  prices: [],
};

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return createElement(QueryClientProvider, { client: queryClient }, children);
}

async function renderForm(onClose = vi.fn()) {
  const rendered = renderHook(() => usePriceConfigForm('c1', court, onClose), { wrapper });
  await waitFor(() => {
    expect(rendered.result.current.form.getValues()).toEqual(EMPTY_PRICE_FORM_VALUES);
  });
  // react-hook-form's formState is a proxy that only re-renders for the keys
  // something has read; the dialog reads `errors` on every render, so the
  // test reads it once up front to subscribe the same way.
  expect(rendered.result.current.form.formState.errors).toEqual({});
  return { ...rendered, onClose };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockSchedules = [];
});

describe('usePriceConfigForm — building the request', () => {
  it("converts pesos to cents and builds each day's band from that day's schedule", async () => {
    vi.mocked(courtsApi.updatePrices).mockResolvedValueOnce({ prices: [] });
    mockSchedules = [schedule({ day: 'monday', open_time: '08:00', close_time: '23:00' })];
    const { result } = await renderForm();

    act(() => {
      result.current.form.setValue('monday', 120);
    });
    act(() => {
      result.current.onSubmit(result.current.form.getValues());
    });

    await waitFor(() => {
      expect(courtsApi.updatePrices).toHaveBeenCalledWith('c1', 'ct1', {
        prices: [{ price: 12000, day_type: 'monday', time_from: '08:00', time_to: '23:00' }],
      });
    });
  });

  // The complex closing after midnight (Thursday 08:00-01:30) is exactly the
  // scenario QA reported: the dialog must build a payload the server accepts
  // (time_to <= time_from means "runs into the next day", the span_min generated column)
  // instead of one it rejects with a silent 422.
  it('builds a valid payload for a day whose schedule crosses midnight', async () => {
    vi.mocked(courtsApi.updatePrices).mockResolvedValueOnce({ prices: [] });
    mockSchedules = [schedule({ day: 'thursday', open_time: '08:00', close_time: '01:30' })];
    const { result } = await renderForm();

    act(() => {
      result.current.form.setValue('thursday', 150);
    });
    act(() => {
      result.current.onSubmit(result.current.form.getValues());
    });

    await waitFor(() => {
      expect(courtsApi.updatePrices).toHaveBeenCalledWith('c1', 'ct1', {
        prices: [{ price: 15000, day_type: 'thursday', time_from: '08:00', time_to: '01:30' }],
      });
    });
  });

  it('rejects a negative price on the field, client-side, without sending a request', async () => {
    const { result } = await renderForm();

    act(() => {
      result.current.form.setValue('monday', -50);
    });
    await act(async () => {
      await result.current.form.handleSubmit(result.current.onSubmit)();
    });

    await waitFor(() => {
      expect(result.current.form.formState.errors.monday?.message).toBe('El precio no puede ser negativo');
    });
    expect(courtsApi.updatePrices).not.toHaveBeenCalled();
  });
});

describe('usePriceConfigForm — server errors', () => {
  // A server 422 field error is keyed by array index ("prices[0].time_to"),
  // not by day name — the form has to trace that index back to the row it
  // came from and put the message there instead of leaving the dialog to
  // fail silently, which is the other half of the QA-reported bug.
  it('maps a server field error back onto the row it came from', async () => {
    const serverError = await makeConsumedHttpError(422, {
      error: { 'prices[0].time_to': 'must be after time_from' },
    });
    vi.mocked(courtsApi.updatePrices).mockRejectedValueOnce(serverError);
    mockSchedules = [schedule({ day: 'thursday', open_time: '08:00', close_time: '01:30' })];
    const { result, onClose } = await renderForm();

    act(() => {
      result.current.form.setValue('thursday', 150);
    });
    act(() => {
      result.current.onSubmit(result.current.form.getValues());
    });

    await waitFor(() => {
      expect(result.current.form.formState.errors.thursday?.message).toBe('must be after time_from');
    });
    expect(onClose).not.toHaveBeenCalled();
    // useUpdatePrices stays quiet on a mappable field error so it is not shown
    // twice — once on the row and once as a toast.
    expect(toast.error).not.toHaveBeenCalled();
  });

  it('falls back to a toast for an error it cannot place on a row', async () => {
    const serverError = await makeConsumedHttpError(422, {
      error: { prices: 'must contain at least one price' },
    });
    vi.mocked(courtsApi.updatePrices).mockRejectedValueOnce(serverError);
    mockSchedules = [schedule({ day: 'monday' })];
    const { result } = await renderForm();

    act(() => {
      result.current.form.setValue('monday', 100);
    });
    act(() => {
      result.current.onSubmit(result.current.form.getValues());
    });

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('must contain at least one price');
    });
  });
});
