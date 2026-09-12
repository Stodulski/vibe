import { act, renderHook, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useClientNotes } from './useClientNotes';
import type { Client } from '@/shared/types/api.types';
import { createQueryWrapper } from '@/test/test-utils';

vi.mock('../../api/clients.api', () => ({
  clientsApi: {
    update: vi.fn(),
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

function makeClient(overrides: Partial<Client> = {}): Client {
  return {
    id: 'cl1',
    complex_id: 'c1',
    first_name: 'Juan',
    last_name: 'Perez',
    phone: '1155550000',
    notes: 'nota original',
    is_blocked: false,
    total_bookings: 0,
    no_shows: 0,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('useClientNotes — A4 autosave error handling', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
  });

  afterEach(async () => {
    vi.useRealTimers();
    const { clientsApi } = await import('../../api/clients.api');
    vi.mocked(clientsApi.update).mockReset();
  });

  // A4: a failed autosave must not discard what the person typed, and must
  // never claim success.
  it('keeps the typed draft and surfaces an inline error when the save fails', async () => {
    const { clientsApi } = await import('../../api/clients.api');
    vi.mocked(clientsApi.update).mockRejectedValueOnce(await makeConsumedHttpError(500, {}));

    const client = makeClient();
    const { result } = renderHook(() => useClientNotes('c1', client), { wrapper: createQueryWrapper() });

    act(() => {
      result.current.onNotesChange('nota en progreso');
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(800);
    });

    await waitFor(() => {
      expect(result.current.saveError).toBe(ES_AR.clients.notesSaveError);
    });
    expect(result.current.notes).toBe('nota en progreso');
    expect(toast.success).not.toHaveBeenCalled();
  });

  // A4: a successful save clears both the error and the pending indicator.
  it('clears the pending state and shows a success toast when the save succeeds', async () => {
    const { clientsApi } = await import('../../api/clients.api');
    vi.mocked(clientsApi.update).mockResolvedValueOnce({
      client: makeClient({ notes: 'nota guardada' }),
    });

    const client = makeClient();
    const { result } = renderHook(() => useClientNotes('c1', client), { wrapper: createQueryWrapper() });

    act(() => {
      result.current.onNotesChange('nota guardada');
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(800);
    });

    await waitFor(() => {
      expect(result.current.isSaving).toBe(false);
    });
    expect(result.current.saveError).toBeUndefined();
    expect(toast.success).toHaveBeenCalledWith(ES_AR.clients.notesUpdated);
  });
});

describe('useClientNotes — A4 retry', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
  });

  afterEach(async () => {
    vi.useRealTimers();
    const { clientsApi } = await import('../../api/clients.api');
    vi.mocked(clientsApi.update).mockReset();
  });

  // A4: a retry re-sends the last attempted value for the same client.
  it('retries the last failed save with the same value', async () => {
    const { clientsApi } = await import('../../api/clients.api');
    vi.mocked(clientsApi.update)
      .mockRejectedValueOnce(await makeConsumedHttpError(500, {}))
      .mockResolvedValueOnce({ client: makeClient({ notes: 'nota en progreso' }) });

    const client = makeClient();
    const { result } = renderHook(() => useClientNotes('c1', client), { wrapper: createQueryWrapper() });

    act(() => {
      result.current.onNotesChange('nota en progreso');
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(800);
    });
    await waitFor(() => {
      expect(result.current.saveError).toBeDefined();
    });

    act(() => {
      result.current.retrySave();
    });
    await waitFor(() => {
      expect(result.current.saveError).toBeUndefined();
    });

    expect(clientsApi.update).toHaveBeenCalledTimes(2);
    expect(clientsApi.update).toHaveBeenLastCalledWith('c1', 'cl1', { notes: 'nota en progreso' });
  });
});

describe('useClientNotes — M1 draft sync on client id change', () => {
  // M1: `useUpdateClient`'s onSuccess invalidates `clients.detail`, which
  // refetches and hands back a NEW `client` reference for the SAME client —
  // syncing on that reference (instead of the id) used to overwrite
  // whatever the person was mid-typing with the stale pre-edit notes.
  it('does not erase a draft when a refetch delivers a new client reference with the old notes', () => {
    const client = makeClient({ notes: 'nota original' });
    const { result, rerender } = renderHook(({ client }: { client: Client }) => useClientNotes('c1', client), {
      wrapper: createQueryWrapper(),
      initialProps: { client },
    });

    act(() => {
      result.current.onNotesChange('nota en progreso');
    });
    expect(result.current.notes).toBe('nota en progreso');

    // Same id, new object reference, stale notes — exactly what a
    // background refetch racing the debounce would deliver.
    rerender({ client: { ...client } });

    expect(result.current.notes).toBe('nota en progreso');
  });

  // Switching to a genuinely different client must still reset the draft.
  it('resets the draft when a different client is selected', () => {
    const clientA = makeClient({ id: 'cl1', notes: 'nota de A' });
    const { result, rerender } = renderHook(({ client }: { client: Client }) => useClientNotes('c1', client), {
      wrapper: createQueryWrapper(),
      initialProps: { client: clientA },
    });

    act(() => {
      result.current.onNotesChange('draft sin guardar');
    });
    expect(result.current.notes).toBe('draft sin guardar');

    const clientB = makeClient({ id: 'cl2', notes: 'nota de B' });
    rerender({ client: clientB });

    expect(result.current.notes).toBe('nota de B');
  });
});
