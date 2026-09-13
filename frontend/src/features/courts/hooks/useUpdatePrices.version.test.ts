import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '@/test/msw/server';
import { makeCourt, makePrice } from '@/test/factories';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CourtWithPrices } from '@/shared/types/api.types';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

function courtWithPrices(overrides: Partial<CourtWithPrices> = {}): CourtWithPrices {
  return { ...makeCourt(overrides), prices: [] };
}

/**
 * Goes through the real `courtsApi` and the real ky client, with MSW
 * standing in for the backend. The price table carries no `version` of its
 * own — this PUT is guarded by the COURT's version instead — so the
 * assertion is that the court's version (not a price's) reaches the wire.
 */
async function renderUpdatePrices(complexId: string, courtId: string, courtVersion: number) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  queryClient.setQueryData(queryKeys.courts.byComplex(complexId), {
    courts: [courtWithPrices({ id: courtId, complex_id: complexId, version: courtVersion })],
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);

  const { useUpdatePrices } = await import('./useUpdatePrices');
  const rendered = renderHook(() => useUpdatePrices(complexId), { wrapper });
  return { ...rendered, queryClient };
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('useUpdatePrices — optimistic concurrency', () => {
  it("sends the court's version already in cache on the PUT body", async () => {
    let receivedBody: unknown;
    server.use(
      http.put('*/complexes/:complexId/courts/:courtId/prices', async ({ request }) => {
        receivedBody = await request.json();
        return HttpResponse.json({ prices: [makePrice()] });
      }),
    );

    const { result } = await renderUpdatePrices('c1', 'ct1', 7);
    result.current.mutate({
      courtId: 'ct1',
      data: { prices: [{ price: 10000, day_type: 'monday', time_from: '08:00', time_to: '23:00' }] },
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(receivedBody).toMatchObject({ version: 7 });
  });

  it('invalidates the court list and toasts a reload notice on a stale-version 409', async () => {
    server.use(
      http.put('*/complexes/:complexId/courts/:courtId/prices', () =>
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

    const { result, queryClient } = await renderUpdatePrices('c1', 'ct1', 7);
    result.current.mutate({
      courtId: 'ct1',
      data: { prices: [{ price: 10000, day_type: 'monday', time_from: '08:00', time_to: '23:00' }] },
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(ES_AR.common.versionConflict);
    expect(queryClient.getQueryState(queryKeys.courts.byComplex('c1'))?.isInvalidated).toBe(true);
  });
});
