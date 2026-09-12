import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSearchParams } from 'react-router-dom';
import { createWrapper } from '@/test/test-utils';
import { useComplexesTableState } from './useComplexesTableState';

const mockGetComplexes = vi.fn();

vi.mock('../../api/admin.api', () => ({
  adminApi: {
    getComplexes: (...args: unknown[]) => mockGetComplexes(...args) as unknown,
  },
}));

function useProbe() {
  const state = useComplexesTableState();
  const [params] = useSearchParams();
  return { state, params };
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  mockGetComplexes.mockResolvedValue({
    complexes: [],
    metadata: { has_more: false, next_cursor: null },
  });
});

afterEach(() => {
  vi.useRealTimers();
});

// Finding M11: search used to live in useState, so leaving for a row's
// detail page and returning with the browser's back button lost it. It now
// lives in the URL as `?search=`.
describe('useComplexesTableState — reading the URL on mount', () => {
  it('defaults to an empty search when the URL carries none', () => {
    const { result } = renderHook(() => useProbe(), {
      wrapper: createWrapper(['/admin/complexes']),
    });

    expect(result.current.state.searchInput).toBe('');
  });

  it('restores the search term from the URL', () => {
    const { result } = renderHook(() => useProbe(), {
      wrapper: createWrapper(['/admin/complexes?search=club-norte']),
    });

    expect(result.current.state.searchInput).toBe('club-norte');
  });
});

describe('useComplexesTableState — writing to the URL', () => {
  it('writes the search term to the URL only after the debounce settles', async () => {
    const { result } = renderHook(() => useProbe(), {
      wrapper: createWrapper(['/admin/complexes']),
    });

    act(() => {
      result.current.state.setSearchInput('club');
    });
    expect(result.current.params.get('search')).toBeNull();

    await act(() => vi.advanceTimersByTimeAsync(300));
    expect(result.current.params.get('search')).toBe('club');
  });

  it('drops the search param instead of writing an empty string', async () => {
    const { result } = renderHook(() => useProbe(), {
      wrapper: createWrapper(['/admin/complexes?search=club-norte']),
    });

    act(() => {
      result.current.state.setSearchInput('');
    });
    await act(() => vi.advanceTimersByTimeAsync(300));
    expect(result.current.params.get('search')).toBeNull();
  });
});
