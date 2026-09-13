import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '@/test/msw/server';
import { makeCourt } from '@/test/factories';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CourtWithPrices } from '@/shared/types/api.types';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

function courtWithPrices(overrides: Partial<CourtWithPrices> = {}): CourtWithPrices {
  return { ...makeCourt(overrides), prices: [] };
}

/**
 * Goes through the real `courtsApi` and the real ky client, with MSW
 * standing in for the backend — the assertion is that `version` reaches the
 * wire, which a mocked api module could only ever confirm as an argument.
 */
async function renderUpdateCourt(complexId: string, courtId: string, version: number) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  queryClient.setQueryData(queryKeys.courts.byComplex(complexId), {
    courts: [courtWithPrices({ id: courtId, complex_id: complexId, version })],
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);

  const { useUpdateCourt } = await import('./useUpdateCourt');
  const rendered = renderHook(() => useUpdateCourt(complexId), { wrapper });
  return { ...rendered, queryClient };
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('useUpdateCourt — optimistic concurrency', () => {
  it('sends the version already in cache on the PUT body', async () => {
    let receivedBody: unknown;
    server.use(
      http.put('*/complexes/:complexId/courts/:courtId', async ({ request }) => {
        receivedBody = await request.json();
        return HttpResponse.json({ court: makeCourt({ id: 'ct1', version: 6, name: 'Cancha renombrada' }) });
      }),
    );

    const { result } = await renderUpdateCourt('c1', 'ct1', 5);
    result.current.mutate({ courtId: 'ct1', data: { name: 'Cancha renombrada' } });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(receivedBody).toMatchObject({ version: 5, name: 'Cancha renombrada' });
  });

  it('invalidates the court list and toasts a reload notice on a stale-version 409', async () => {
    server.use(
      http.put('*/complexes/:complexId/courts/:courtId', () =>
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

    const { result, queryClient } = await renderUpdateCourt('c1', 'ct1', 5);
    result.current.mutate({ courtId: 'ct1', data: { name: 'Cancha renombrada' } });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(ES_AR.common.versionConflict);
    expect(queryClient.getQueryState(queryKeys.courts.byComplex('c1'))?.isInvalidated).toBe(true);
  });
});
