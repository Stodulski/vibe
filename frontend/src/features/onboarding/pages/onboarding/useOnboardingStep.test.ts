import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useOnboardingStep } from './useOnboardingStep';
import type { Complex } from '@/shared/types/api.types';

const navigateMock = vi.fn();

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>();
  return { ...actual, useNavigate: () => navigateMock };
});

const complex = { id: 'c1', mp_user_id: null } as unknown as Complex;
const connectedComplex = { id: 'c1', mp_user_id: 'mp1' } as unknown as Complex;

function baseArgs(overrides: Partial<Parameters<typeof useOnboardingStep>[0]> = {}) {
  return {
    complexId: null,
    currentComplex: null,
    courts: undefined,
    complexesLoading: false,
    courtsLoading: false,
    ...overrides,
  };
}

describe('useOnboardingStep — deriving the step from server data', () => {
  it('shows no step while the complexes list is still loading', () => {
    const { result } = renderHook(() => useOnboardingStep(baseArgs({ complexesLoading: true })));
    expect(result.current.step).toBeNull();
  });

  it('opens step 1 once loaded with no complex yet', () => {
    const { result } = renderHook(() => useOnboardingStep(baseArgs()));
    expect(result.current.step).toBe(1);
  });

  it('shows no step while courts are still loading for an existing complex', () => {
    const { result } = renderHook(() => useOnboardingStep(baseArgs({ complexId: 'c1', courtsLoading: true })));
    expect(result.current.step).toBeNull();
  });

  it('opens step 2 once a complex exists with no courts', () => {
    const { result } = renderHook(() => useOnboardingStep(baseArgs({ complexId: 'c1', courts: [] })));
    expect(result.current.step).toBe(2);
  });

  it('opens step 3 once a complex has courts but no MercadoPago connection', () => {
    const { result } = renderHook(() =>
      useOnboardingStep(
        baseArgs({
          complexId: 'c1',
          courts: [{}],
          currentComplex: complex,
        }),
      ),
    );
    expect(result.current.step).toBe(3);
  });

  it('completes onboarding once fully set up, instead of showing a step', () => {
    renderHook(() =>
      useOnboardingStep(
        baseArgs({
          complexId: 'c1',
          courts: [{}],
          currentComplex: connectedComplex,
        }),
      ),
    );
    expect(navigateMock).toHaveBeenCalledWith('/dashboard', { replace: true });
  });
});

describe('useOnboardingStep — pinning the step within a session', () => {
  it('keeps step 2 when creating the first court refetches courts and would derive step 3', () => {
    const { result, rerender } = renderHook(
      (props: Parameters<typeof useOnboardingStep>[0]) => useOnboardingStep(props),
      {
        initialProps: baseArgs({ complexId: 'c1', courts: [], currentComplex: complex }),
      },
    );
    expect(result.current.step).toBe(2);

    // The court was created: the courts query refetches and now has one
    // court, which on its own would derive step 3. The owner never asked to
    // move on (that's the "Siguiente" button, tested via `changeStep`), so
    // the page must stay on step 2.
    rerender(baseArgs({ complexId: 'c1', courts: [{}], currentComplex: complex }));
    expect(result.current.step).toBe(2);
  });
});

describe('useOnboardingStep — manual overrides', () => {
  it('keeps a manually chosen step even while the server data would derive a different one', () => {
    const { result, rerender } = renderHook(
      (props: Parameters<typeof useOnboardingStep>[0]) => useOnboardingStep(props),
      {
        initialProps: baseArgs({ complexId: 'c1', courts: [{}], currentComplex: complex }),
      },
    );
    expect(result.current.step).toBe(3);

    act(() => {
      result.current.changeStep(1);
    });
    // Re-render with the exact same (still step-3-implying) server data —
    // the manual "back" to step 1 must not be overwritten by the derivation.
    rerender(baseArgs({ complexId: 'c1', courts: [{}], currentComplex: complex }));
    expect(result.current.step).toBe(1);
  });

  it('drops the manual override once a genuinely different complex loads', () => {
    const { result, rerender } = renderHook(
      (props: Parameters<typeof useOnboardingStep>[0]) => useOnboardingStep(props),
      {
        initialProps: baseArgs({ complexId: 'c1', courts: [{}], currentComplex: complex }),
      },
    );

    act(() => {
      result.current.changeStep(1);
    });
    expect(result.current.step).toBe(1);

    // A different complex (e.g. "Agregar otro complejo") starts over.
    rerender(baseArgs({ complexId: 'c2', courts: [], currentComplex: null }));
    expect(result.current.step).toBe(2);
  });
});
