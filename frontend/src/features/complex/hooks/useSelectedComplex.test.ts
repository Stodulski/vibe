import { renderHook } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { makeComplex } from '@/test/factories';
import { useSelectedComplex } from './useSelectedComplex';

const useComplexesMock = vi.fn();

vi.mock('./useComplexes', () => ({
  useComplexes: () => useComplexesMock() as unknown,
}));

describe('useSelectedComplex', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('derives the account single complex from the complexes query', () => {
    useComplexesMock.mockReturnValue({ data: [makeComplex({ id: 'c1' })], isLoading: false });

    const { result } = renderHook(() => useSelectedComplex());

    expect(result.current.complex?.id).toBe('c1');
    expect(result.current.selectedComplexId).toBe('c1');
    expect(result.current.isLoading).toBe(false);
    expect(result.current.needsOnboarding).toBe(false);
  });

  it('needs onboarding once loaded with no complex at all', () => {
    useComplexesMock.mockReturnValue({ data: [], isLoading: false });

    const { result } = renderHook(() => useSelectedComplex());

    expect(result.current.complex).toBeNull();
    expect(result.current.selectedComplexId).toBeNull();
    expect(result.current.needsOnboarding).toBe(true);
  });

  it('does not claim onboarding is needed while still loading', () => {
    useComplexesMock.mockReturnValue({ data: undefined, isLoading: true });

    const { result } = renderHook(() => useSelectedComplex());

    expect(result.current.needsOnboarding).toBe(false);
    expect(result.current.isLoading).toBe(true);
  });
});
