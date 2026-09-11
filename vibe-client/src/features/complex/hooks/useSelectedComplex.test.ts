import { renderHook } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { makeComplex } from '@/test/factories';
import { useSelectedComplex } from './useSelectedComplex';

const mockSetSelectedComplexId = vi.fn();
let mockSelectedComplexId: string | null = null;

vi.mock('@/shared/stores', () => ({
  useStore: (
    selector: (s: {
      selectedComplexId: string | null;
      setSelectedComplexId: typeof mockSetSelectedComplexId;
    }) => unknown,
  ) =>
    selector({
      selectedComplexId: mockSelectedComplexId,
      setSelectedComplexId: mockSetSelectedComplexId,
    }),
}));

vi.mock('./useComplexes', () => ({
  useComplexes: () => ({
    data: [makeComplex({ id: 'c1' }), makeComplex({ id: 'c2' })],
    isLoading: false,
  }),
}));

describe('useSelectedComplex', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSelectedComplexId = null;
  });

  // The old implementation used a `useEffect` to persist a default pick back
  // into the store, which caused an extra render (see `isReady` in the old
  // source) and wrote a choice the owner never made. Deriving the effective
  // id in render means the first complex is already correct on the very
  // first render, with nothing written to the store for it.
  it('derives the first complex without writing a default back to the store', () => {
    const { result } = renderHook(() => useSelectedComplex());

    expect(result.current.complex?.id).toBe('c1');
    expect(result.current.selectedComplexId).toBe('c1');
    expect(result.current.isLoading).toBe(false);
    expect(mockSetSelectedComplexId).not.toHaveBeenCalled();
  });

  it('falls back to the first complex when the stored id no longer names one', () => {
    mockSelectedComplexId = 'deleted-complex';
    const { result } = renderHook(() => useSelectedComplex());

    expect(result.current.complex?.id).toBe('c1');
    expect(result.current.selectedComplexId).toBe('c1');
  });

  it('keeps the stored complex when it still exists', () => {
    mockSelectedComplexId = 'c2';
    const { result } = renderHook(() => useSelectedComplex());

    expect(result.current.complex?.id).toBe('c2');
    expect(result.current.selectedComplexId).toBe('c2');
  });
});
