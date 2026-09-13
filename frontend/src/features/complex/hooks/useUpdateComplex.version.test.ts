import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '@/test/msw/server';
import { makeComplex } from '@/test/factories';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

/**
 * Goes through the real `complexApi` and the real ky client, with MSW
 * standing in for the backend — the assertion is that `version` reaches the
 * wire, which a mocked api module could only ever confirm as an argument.
 */
async function renderUpdateComplex(complexId: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  queryClient.setQueryData(queryKeys.complexes.detail(complexId), {
    complex: makeComplex({ id: complexId, version: 3 }),
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);

  const { useUpdateComplex } = await import('./useUpdateComplex');
  const rendered = renderHook(() => useUpdateComplex(complexId), { wrapper });
  return { ...rendered, queryClient };
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('useUpdateComplex — optimistic concurrency', () => {
  it('sends the version already in cache on the PUT body', async () => {
    let receivedBody: unknown;
    server.use(
      http.put('*/complexes/:complexId', async ({ request }) => {
        receivedBody = await request.json();
        return HttpResponse.json({ complex: makeComplex({ id: 'c1', version: 4, name: 'Club Nuevo' }) });
      }),
    );

    const { result } = await renderUpdateComplex('c1');
    result.current.mutate({ name: 'Club Nuevo' });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(receivedBody).toMatchObject({ version: 3, name: 'Club Nuevo' });
  });

  it('invalidates the cached complex and toasts a reload notice on a stale-version 409', async () => {
    server.use(
      http.put('*/complexes/:complexId', () =>
        HttpResponse.json(
          {
            type: 'https://vibe.com.ar/problems/stale-version',
            title: 'Stale Version',
            status: 409,
            detail: 'unable to update the record due to an edit conflict, please try again',
          },
          { status: 409 },
        ),
      ),
    );

    const { result, queryClient } = await renderUpdateComplex('c1');
    result.current.mutate({ name: 'Club Nuevo' });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(ES_AR.common.versionConflict);
    expect(queryClient.getQueryState(queryKeys.complexes.detail('c1'))?.isInvalidated).toBe(true);
  });

  // A generic 409 (a duplicate slug, say) is not this — it must keep the
  // ordinary error toast, not silently become a "reload" prompt.
  it('does not treat a generic conflict as a stale-version refusal', async () => {
    server.use(
      http.put('*/complexes/:complexId', () =>
        HttpResponse.json(
          { type: 'https://vibe.com.ar/problems/conflict', title: 'Conflict', status: 409, detail: 'slug_taken' },
          { status: 409 },
        ),
      ),
    );

    const { result } = await renderUpdateComplex('c1');
    result.current.mutate({ name: 'Club Nuevo' });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.common.versionConflict);
  });
});
