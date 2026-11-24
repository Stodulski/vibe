import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { makeComplex } from '@/test/factories';
import { queryKeys } from '@/shared/lib/queryKeys';
import type { Complex } from '@/shared/types/api.types';
import { useComplexes } from '../../hooks/useComplexes';
import { useComplexForm } from './useComplexForm';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const staleVersion = {
  type: 'https://vibe.com.ar/problems/stale-version',
  title: 'Stale Version',
  status: 409,
  detail: 'unable to update the record due to an edit conflict, please try again',
};

/**
 * Settings and onboarding fill the form from the complexes LIST, never from
 * the detail entry, so the harness does the same: only `complexes.all` is in
 * cache. Real hooks, real ky client, MSW as the backend — the assertion is on
 * the `version` that reaches the wire.
 */
function renderSettingsForm(complex: Complex) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity }, mutations: { retry: false } },
  });
  queryClient.setQueryData(queryKeys.complexes.all, { complexes: [complex] });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const rendered = renderHook(
    () => {
      const { data } = useComplexes();
      return useComplexForm({ complex: data?.[0] });
    },
    { wrapper },
  );
  return { ...rendered, queryClient };
}

function submit(result: { current: ReturnType<typeof useComplexForm> }) {
  act(() => {
    result.current.onSubmit(result.current.form.getValues());
  });
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('useComplexForm — version of the copy on screen', () => {
  it('sends the version of the complex the form was filled from', async () => {
    const bodies: Record<string, unknown>[] = [];
    server.use(
      http.put('*/complexes/:complexId', async ({ request }) => {
        bodies.push((await request.json()) as Record<string, unknown>);
        return HttpResponse.json({ complex: makeComplex({ version: 4 }) });
      }),
    );

    const { result } = renderSettingsForm(makeComplex({ version: 3 }));
    submit(result);

    await waitFor(() => {
      expect(bodies).toHaveLength(1);
    });
    expect(bodies[0]).toMatchObject({ version: 3 });
  });

  it('refills the form with the fresh copy after a stale-version 409, so the next save succeeds', async () => {
    const bodies: Record<string, unknown>[] = [];
    server.use(
      http.put('*/complexes/:complexId', async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        bodies.push(body);
        if (body.version !== 5) return HttpResponse.json(staleVersion, { status: 409 });
        return HttpResponse.json({ complex: makeComplex({ version: 6, name: 'Club Remoto' }) });
      }),
      http.get('*/complexes', () =>
        HttpResponse.json({ complexes: [makeComplex({ version: 5, name: 'Club Remoto' })] }),
      ),
    );

    const { result } = renderSettingsForm(makeComplex({ version: 3, name: 'Club Padel' }));
    act(() => {
      result.current.form.setValue('name', 'Club Local', { shouldDirty: true });
    });
    submit(result);

    await waitFor(() => {
      expect(result.current.form.getValues('name')).toBe('Club Remoto');
    });

    submit(result);
    await waitFor(() => {
      expect(bodies).toHaveLength(2);
    });
    expect(bodies[1]).toMatchObject({ version: 5 });
  });
});

describe('useComplexForm — a newer copy mid-edit', () => {
  // A focus refetch can bring a copy someone else saved while this owner is
  // typing. Adopting its version silently would let this save overwrite that
  // edit — exactly what the version check exists to refuse.
  it('keeps the edit and its original version when a newer copy arrives mid-edit', async () => {
    const bodies: Record<string, unknown>[] = [];
    server.use(
      http.put('*/complexes/:complexId', async ({ request }) => {
        bodies.push((await request.json()) as Record<string, unknown>);
        return HttpResponse.json(staleVersion, { status: 409 });
      }),
    );

    const { result, queryClient } = renderSettingsForm(makeComplex({ version: 3 }));
    act(() => {
      result.current.form.setValue('name', 'Club Local', { shouldDirty: true });
    });
    act(() => {
      queryClient.setQueryData(queryKeys.complexes.all, {
        complexes: [makeComplex({ version: 5, name: 'Club Remoto' })],
      });
    });

    expect(result.current.form.getValues('name')).toBe('Club Local');
    submit(result);
    await waitFor(() => {
      expect(bodies).toHaveLength(1);
    });
    expect(bodies[0]).toMatchObject({ version: 3, name: 'Club Local' });
  });
});
