import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useSlugAvailability } from './useSlugAvailability';
import { createWrapper } from '@/test/test-utils';

const mockSlugAvailable = vi.fn();

vi.mock('../../api/complex.api', () => ({
  complexApi: {
    slugAvailable: (...args: unknown[]) => mockSlugAvailable(...args) as unknown,
  },
}));

// Finding M1: this used to be a hand-rolled `setTimeout` + `AbortController`
// fetch inside a `useEffect` — exactly the pattern the guidelines ask to
// avoid. It is now a debounced `useQuery`.
// Hoisted to file scope (rather than nested in each describe below) partly
// to keep those describe callbacks' own line counts under the repo's
// max-lines-per-function cap, which counts a nested `beforeEach`/`afterEach`'s
// lines as part of the enclosing describe.
beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.clearAllMocks();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useSlugAvailability', () => {
  it('stays idle without querying while the slug is empty or equals currentSlug', async () => {
    const { result, rerender } = renderHook(
      ({ slug }: { slug: string | undefined }) => useSlugAvailability(slug, 'club-norte'),
      {
        wrapper: createWrapper(),
        initialProps: { slug: undefined as string | undefined },
      },
    );
    expect(result.current).toEqual({ status: 'idle' });

    rerender({ slug: 'club-norte' });
    await act(() => vi.advanceTimersByTimeAsync(500));

    expect(result.current).toEqual({ status: 'idle' });
    expect(mockSlugAvailable).not.toHaveBeenCalled();
  });

  it('debounces and reports "checking" before resolving to "free"', async () => {
    mockSlugAvailable.mockResolvedValue({ slug: 'club-sur', valid: true, available: true });

    const { result, rerender } = renderHook(({ slug }: { slug: string | undefined }) => useSlugAvailability(slug), {
      wrapper: createWrapper(),
      initialProps: { slug: 'club-s' },
    });

    rerender({ slug: 'club-su' });
    rerender({ slug: 'club-sur' });

    // Still within the debounce window: no request fired yet.
    await act(() => vi.advanceTimersByTimeAsync(300));
    expect(mockSlugAvailable).not.toHaveBeenCalled();
    expect(result.current.status).toBe('checking');

    await act(() => vi.advanceTimersByTimeAsync(200));
    await waitFor(() => {
      expect(result.current).toEqual({ status: 'free' });
    });

    expect(mockSlugAvailable).toHaveBeenCalledTimes(1);
    expect(mockSlugAvailable).toHaveBeenCalledWith('club-sur', expect.anything());
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call.
describe('useSlugAvailability — taken and error outcomes', () => {
  it('reports "taken" with the server suggestion', async () => {
    mockSlugAvailable.mockResolvedValue({
      slug: 'club-norte',
      valid: true,
      available: false,
      suggestion: 'club-norte-2',
    });

    const { result } = renderHook(() => useSlugAvailability('club-norte'), {
      wrapper: createWrapper(),
    });

    await act(() => vi.advanceTimersByTimeAsync(400));
    await waitFor(() => {
      expect(result.current).toEqual({ status: 'taken', suggestion: 'club-norte-2' });
    });
  });

  it('falls back to "idle" when the check fails, instead of reporting a collision', async () => {
    mockSlugAvailable.mockRejectedValue(new Error('network error'));

    const { result } = renderHook(() => useSlugAvailability('club-norte'), {
      wrapper: createWrapper(),
    });

    await act(() => vi.advanceTimersByTimeAsync(400));
    await waitFor(() => {
      expect(mockSlugAvailable).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(result.current).toEqual({ status: 'idle' });
    });
  });
});
