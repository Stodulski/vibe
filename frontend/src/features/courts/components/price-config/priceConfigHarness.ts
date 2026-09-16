import { expect, vi } from 'vitest';
import { createElement, type ReactNode } from 'react';
import { act, renderHook } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { usePriceConfigForm } from './usePriceConfigForm';
import type { CourtPrice, CourtWithPrices, Schedule } from '@/shared/types/api.types';

/**
 * Shared setup for the three `usePriceConfigForm` specs.
 *
 * The specs are split by what they are about — building the request, refusing
 * a bad week, placing a server error — and each would otherwise repeat forty
 * lines of court, schedule and query-client scaffolding. The `vi.mock` calls
 * stay in the spec files: they are hoisted per file and cannot be shared.
 */

export function schedule(overrides: Partial<Schedule>): Schedule {
  return {
    id: 's1',
    complex_id: 'c1',
    day: 'monday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
    ...overrides,
  };
}

export function makeCourt(prices: CourtPrice[] = []): CourtWithPrices {
  return {
    id: 'ct1',
    complex_id: 'c1',
    name: 'Cancha 1',
    sport: 'padel',
    court_type: 'outdoor',
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices,
  };
}

export function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return createElement(QueryClientProvider, { client: queryClient }, children);
}

export function renderForm({ schedules = [] as Schedule[], court = makeCourt(), onClose = vi.fn() } = {}) {
  const rendered = renderHook(() => usePriceConfigForm('c1', court, schedules, onClose), { wrapper });
  // react-hook-form's formState is a proxy that only re-renders for the keys
  // something has read; the dialog reads `errors` on every render, so the
  // harness reads it once up front to subscribe the same way.
  expect(rendered.result.current.form.formState.errors).toEqual({});
  return { ...rendered, onClose };
}

/** Runs the real submit path, resolver included — what pressing Guardar does. */
export async function submit(result: { current: ReturnType<typeof usePriceConfigForm> }): Promise<void> {
  await act(async () => {
    await result.current.form.handleSubmit(result.current.onSubmit)();
  });
}
