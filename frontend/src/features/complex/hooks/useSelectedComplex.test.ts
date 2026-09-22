import { renderHook } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { makeComplex } from '@/test/factories';
import { useSelectedComplex } from './useSelectedComplex';

const useComplexesMock = vi.fn();

vi.mock('./useComplexes', () => ({
  useComplexes: () => useComplexesMock() as unknown,
}));

// Trims each test's mock to only what it varies — `useComplexes` always
// returns this full shape, and every test below overrides just the fields
// its scenario cares about.
function mockComplexesQuery(overrides: Record<string, unknown>) {
  useComplexesMock.mockReturnValue({
    data: undefined,
    isLoading: false,
    isSuccess: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    ...overrides,
  });
}

describe('useSelectedComplex', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('derives the account single complex from the complexes query', () => {
    mockComplexesQuery({ data: [makeComplex({ id: 'c1' })], isSuccess: true });

    const { result } = renderHook(() => useSelectedComplex());

    expect(result.current.complex?.id).toBe('c1');
    expect(result.current.selectedComplexId).toBe('c1');
    expect(result.current.isLoading).toBe(false);
    expect(result.current.needsOnboarding).toBe(false);
  });

  it('needs onboarding once loaded successfully with no complex at all', () => {
    mockComplexesQuery({ data: [], isSuccess: true });

    const { result } = renderHook(() => useSelectedComplex());

    expect(result.current.complex).toBeNull();
    expect(result.current.selectedComplexId).toBeNull();
    expect(result.current.needsOnboarding).toBe(true);
  });

  it('does not claim onboarding is needed while still loading', () => {
    mockComplexesQuery({ isLoading: true });

    const { result } = renderHook(() => useSelectedComplex());

    expect(result.current.needsOnboarding).toBe(false);
    expect(result.current.isLoading).toBe(true);
  });

  it('does not claim onboarding is needed when the query errored', () => {
    const error = new Error('Network error');
    mockComplexesQuery({ isError: true, error });

    const { result } = renderHook(() => useSelectedComplex());

    expect(result.current.needsOnboarding).toBe(false);
    expect(result.current.isError).toBe(true);
    expect(result.current.error).toBe(error);
  });
});
