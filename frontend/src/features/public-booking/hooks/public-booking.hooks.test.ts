import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { toast } from 'sonner';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';

// Shared default `createBooking` resolution: the module-registry note on the
// `afterEach` below explains why this needs restoring after every test.
// `vi.hoisted` because `vi.mock` factories run before normal top-level code.
const { DEFAULT_CREATE_BOOKING_RESPONSE } = vi.hoisted(() => ({
  DEFAULT_CREATE_BOOKING_RESPONSE: {
    booking: {
      status: 'pending' as const,
      collection_status: 'unpaid' as const,
      refund_status: 'none' as const,
      date: '2026-03-18',
      start_time: '10:00',
      starts_at: '2026-03-18T10:00:00-03:00',
      ends_at: '2026-03-18T11:30:00-03:00',
      court_name: 'Cancha 1',
      complex_name: 'Club Norte',
      price: 1_000_000,
      deposit_amount: 300_000,
    },
    token: 't1',
    mp_init_point: 'https://mp.com',
  },
}));

vi.mock('../api/public-booking.api', () => ({
  publicBookingApi: {
    getComplex: vi.fn().mockResolvedValue({ complex: { name: 'Club' }, courts: [], schedules: [] }),
    getAvailability: vi.fn().mockResolvedValue({ availability: { slots: [] } }),
    createBooking: vi.fn().mockResolvedValue(DEFAULT_CREATE_BOOKING_RESPONSE),
    getBookingStatus: vi.fn().mockResolvedValue({
      booking: { status: 'confirmed', collection_status: 'fully_paid', refund_status: 'none' },
    }),
  },
}));

vi.mock('@/shared/lib/queryKeys', () => ({
  queryKeys: {
    publicComplex: { bySlug: (slug: string) => ['publicComplex', slug] },
    availability: {
      bySlugAndDate: (slug: string, date: string, duration: number) => ['availability', slug, date, duration],
    },
    bookingStatus: { byId: (id: string) => ['bookingStatus', id] },
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe('useComplexBySlug', () => {
  it('fetches complex by slug', async () => {
    const { useComplexBySlug } = await import('./useComplexBySlug');
    const { result } = renderHook(() => useComplexBySlug('test-club'), {
      wrapper: createWrapper(),
    });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('is disabled when slug is undefined', async () => {
    const { useComplexBySlug } = await import('./useComplexBySlug');
    const { result } = renderHook(() => useComplexBySlug(undefined), { wrapper: createWrapper() });
    expect(result.current.fetchStatus).toBe('idle');
  });
});

describe('useAvailability', () => {
  it('fetches availability', async () => {
    const { useAvailability } = await import('./useAvailability');
    const { result } = renderHook(() => useAvailability('test-club', '2026-03-18', 90), {
      wrapper: createWrapper(),
    });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toEqual({ slots: [] });
  });

  it('is disabled when slug is undefined', async () => {
    const { useAvailability } = await import('./useAvailability');
    const { result } = renderHook(() => useAvailability(undefined, '2026-03-18', 90), {
      wrapper: createWrapper(),
    });
    expect(result.current.fetchStatus).toBe('idle');
  });
});

describe('useBookingStatus', () => {
  it('fetches booking status', async () => {
    const { useBookingStatus } = await import('./useBookingStatus');
    const { result } = renderHook(() => useBookingStatus('t1'), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.status).toBe('confirmed');
  });

  it('is disabled when token is null', async () => {
    const { useBookingStatus } = await import('./useBookingStatus');
    const { result } = renderHook(() => useBookingStatus(null), { wrapper: createWrapper() });
    expect(result.current.fetchStatus).toBe('idle');
  });

  it('has timedOut property', async () => {
    const { useBookingStatus } = await import('./useBookingStatus');
    const { result } = renderHook(() => useBookingStatus('t1'), { wrapper: createWrapper() });
    expect(result.current.timedOut).toBe(false);
  });
});

describe('usePublicBooking', () => {
  it('returns a mutation', async () => {
    const { usePublicBooking } = await import('./usePublicBooking');
    const { result } = renderHook(() => usePublicBooking(), { wrapper: createWrapper() });
    expect(typeof result.current.mutate).toBe('function');
  });

  it('onError surfaces the real backend message from error.data instead of the generic fallback', async () => {
    const { publicBookingApi } = await import('../api/public-booking.api');
    const backendError = await makeConsumedHttpError(400, {
      error: 'El telefono ya tiene una reserva pendiente',
    });
    vi.mocked(publicBookingApi.createBooking).mockRejectedValueOnce(backendError);

    const { usePublicBooking } = await import('./usePublicBooking');
    const { result } = renderHook(() => usePublicBooking(), { wrapper: createWrapper() });

    result.current.mutate({
      complex_id: 'c1',
      court_id: 'ct1',
      date: '2026-03-18',
      start_time: '10:00',
      duration_minutes: 90,
      client_first_name: 'Juan',
      client_last_name: 'Perez',
      client_phone: '1155550000',
      client_email: 'juan@test.com',
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith('El telefono ya tiene una reserva pendiente');
    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.publicBooking.bookingCreateError);
  });

  it('onError falls back to the generic i18n message when the backend body has no error field', async () => {
    const { publicBookingApi } = await import('../api/public-booking.api');
    const backendError = await makeConsumedHttpError(500, {});
    vi.mocked(publicBookingApi.createBooking).mockRejectedValueOnce(backendError);

    const { usePublicBooking } = await import('./usePublicBooking');
    const { result } = renderHook(() => usePublicBooking(), { wrapper: createWrapper() });

    result.current.mutate({
      complex_id: 'c1',
      court_id: 'ct1',
      date: '2026-03-18',
      start_time: '10:00',
      duration_minutes: 90,
      client_first_name: 'Juan',
      client_last_name: 'Perez',
      client_phone: '1155550000',
      client_email: 'juan@test.com',
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(ES_AR.publicBooking.bookingCreateError);
  });

  // `isolate: false` (vitest.config.ts) shares the module registry across
  // test files in the same worker. `mockRejectedValueOnce` above normally
  // self-clears after one call, but explicitly restoring the resolved
  // default here guards against leaking a rejected `createBooking` into
  // BookConfirmPage.test.tsx / BookCancelPage.test.tsx, which mock the same
  // resolved module path.
  afterEach(async () => {
    const { publicBookingApi } = await import('../api/public-booking.api');
    vi.mocked(publicBookingApi.createBooking).mockResolvedValue(DEFAULT_CREATE_BOOKING_RESPONSE);
  });
});
