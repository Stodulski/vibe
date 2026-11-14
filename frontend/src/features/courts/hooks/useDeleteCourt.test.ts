import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { createQueryWrapper } from '@/test/test-utils';

vi.mock('../api/courts.api', () => ({ courtsApi: { delete: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

async function failDeleteWith(status: number, detail: string) {
  const { courtsApi } = await import('../api/courts.api');
  vi.mocked(courtsApi.delete).mockRejectedValueOnce(await makeConsumedHttpError(status, { title: 'Conflict', detail }));
  const { useDeleteCourt } = await import('./useDeleteCourt');
  const { result } = renderHook(() => useDeleteCourt('c1'), { wrapper: createQueryWrapper() });
  result.current.mutate('court-1');
  await waitFor(() => {
    expect(result.current.isError).toBe(true);
  });
}

describe('useDeleteCourt — onError', () => {
  it('says in Spanish that fixed weekly bookings block the delete', async () => {
    await failDeleteWith(
      409,
      'cannot delete court while it has fixed weekly bookings (turnos fijos), cancel them first',
    );
    expect(toast.error).toHaveBeenCalledWith(ES_AR.courts.deleteHasFixedBookings);
  });

  it('keeps showing the server message for any other refusal', async () => {
    await failDeleteWith(409, 'cannot delete court while it has active bookings, cancel them first');
    expect(toast.error).toHaveBeenCalledWith('cannot delete court while it has active bookings, cancel them first');
  });
});
