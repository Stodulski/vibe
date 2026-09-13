import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { createQueryWrapper } from '@/test/test-utils';

vi.mock('../api/courts.api', () => ({
  courtsApi: {
    blockSlot: vi.fn(),
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

describe('useBlockCourtSlot — onError surfaces the real backend message', () => {
  it('shows the backend error message from error.data instead of the generic fallback', async () => {
    const { courtsApi } = await import('../api/courts.api');
    const backendError = await makeConsumedHttpError(400, {
      title: 'Bad Request',
      detail: 'El horario ya tiene una reserva activa',
    });
    vi.mocked(courtsApi.blockSlot).mockRejectedValueOnce(backendError);

    const { useBlockCourtSlot } = await import('./useBlockCourtSlot');
    const { result } = renderHook(() => useBlockCourtSlot('c1'), { wrapper: createQueryWrapper() });

    result.current.mutate({
      courtId: 'ct1',
      data: { date: '2026-03-18', start_time: '10:00', end_time: '11:00' },
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith('El horario ya tiene una reserva activa');
    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.courts.blockError);
  });

  it('falls back to the generic i18n message when the backend body has no error field', async () => {
    const { courtsApi } = await import('../api/courts.api');
    const backendError = await makeConsumedHttpError(500, {});
    vi.mocked(courtsApi.blockSlot).mockRejectedValueOnce(backendError);

    const { useBlockCourtSlot } = await import('./useBlockCourtSlot');
    const { result } = renderHook(() => useBlockCourtSlot('c1'), { wrapper: createQueryWrapper() });

    result.current.mutate({
      courtId: 'ct1',
      data: { date: '2026-03-18', start_time: '10:00', end_time: '11:00' },
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(ES_AR.courts.blockError);
  });

  // `isolate: false` (vitest.config.ts) shares the module registry across
  // test files in the same worker; explicitly resetting here guards
  // against leaking a rejected `blockSlot` mock into any other file that
  // mocks this same resolved module path.
  afterEach(async () => {
    const { courtsApi } = await import('../api/courts.api');
    vi.mocked(courtsApi.blockSlot).mockReset();
  });
});
