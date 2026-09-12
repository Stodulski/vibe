import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { toast } from 'sonner';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { HTTPError } from 'ky';

vi.mock('../api/clients.api', () => ({
  clientsApi: {
    update: vi.fn(),
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

/** Mutates once with a rejected `clientsApi.update` and waits for the error state. */
async function triggerUpdateError(backendError: HTTPError) {
  const { clientsApi } = await import('../api/clients.api');
  vi.mocked(clientsApi.update).mockRejectedValueOnce(backendError);

  const { useUpdateClient } = await import('./useUpdateClient');
  const { result } = renderHook(() => useUpdateClient('c1'), { wrapper: createWrapper() });

  result.current.mutate({ clientId: 'cl1', data: { notes: 'algo' } });

  // A generous timeout, paired with a longer per-test timeout below: this
  // repo's sandboxes run several agents' tsc, eslint and vitest processes
  // concurrently, which can push a plain setTimeout-polled `waitFor` well
  // past the default 1s window under load.
  await waitFor(
    () => {
      expect(result.current.isError).toBe(true);
    },
    { timeout: 25000 },
  );
}

describe('useUpdateClient — onError toasts the real backend message', () => {
  afterEach(async () => {
    const { clientsApi } = await import('../api/clients.api');
    vi.mocked(clientsApi.update).mockReset();
  });

  it('shows the backend error message from error.data instead of the generic fallback', async () => {
    await triggerUpdateError(
      await makeConsumedHttpError(400, {
        error: 'El teléfono no es válido',
      }),
    );

    expect(toast.error).toHaveBeenCalledWith('El teléfono no es válido');
    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.clients.updateError);
  }, 30000);

  it('falls back to the generic i18n message when the backend body has no error field', async () => {
    await triggerUpdateError(await makeConsumedHttpError(500, {}));

    expect(toast.error).toHaveBeenCalledWith(ES_AR.clients.updateError);
  }, 30000);

  // A dropped connection throws a plain TypeError with no `.response` —
  // getHttpErrorMessage must not crash reading it and must still surface a
  // toast instead of leaving the mutation in a broken, silent state. It's
  // the generic connectivity message, not this mutation's own fallback
  // (ERR-04): "we couldn't reach you" beats "we couldn't update this client"
  // for something that never reached the server at all.
  it('shows the generic connectivity toast instead of crashing on a network error', async () => {
    await triggerUpdateError(
      new TypeError('Failed to fetch') as unknown as Awaited<ReturnType<typeof makeConsumedHttpError>>,
    );

    expect(toast.error).toHaveBeenCalledWith(ES_AR.common.networkError);
  }, 30000);
});
