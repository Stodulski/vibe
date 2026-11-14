import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { createQueryWrapper } from '@/test/test-utils';

vi.mock('../api/complex.api', () => ({ complexApi: { delete: vi.fn() } }));
vi.mock('react-router-dom', () => ({ useNavigate: () => vi.fn() }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

async function failDeleteWith(status: number, detail: string) {
  const { complexApi } = await import('../api/complex.api');
  vi.mocked(complexApi.delete).mockRejectedValueOnce(
    await makeConsumedHttpError(status, { title: 'Conflict', detail }),
  );
  const { useDeleteComplex } = await import('./useDeleteComplex');
  const { result } = renderHook(() => useDeleteComplex(), { wrapper: createQueryWrapper() });
  result.current.mutate('c1');
  await waitFor(() => {
    expect(result.current.isError).toBe(true);
  });
}

describe('useDeleteComplex — onError', () => {
  it('says in Spanish that fixed weekly bookings block the delete', async () => {
    await failDeleteWith(
      409,
      'cannot delete the complex while it has fixed weekly bookings (turnos fijos), cancel them first',
    );
    expect(toast.error).toHaveBeenCalledWith(ES_AR.complex.deleteHasFixedBookings);
  });

  it('keeps showing the server message for any other refusal', async () => {
    await failDeleteWith(409, 'cannot delete complex while it has active bookings, cancel them first');
    expect(toast.error).toHaveBeenCalledWith('cannot delete complex while it has active bookings, cancel them first');
  });
});
