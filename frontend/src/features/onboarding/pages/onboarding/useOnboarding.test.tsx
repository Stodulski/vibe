import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import type { ReactNode } from 'react';
import { useOnboarding } from './useOnboarding';
import type { Complex } from '@/shared/types/api.types';

const complexesMock = vi.fn();
const courtsMock = vi.fn();

vi.mock('@/features/complex', () => ({ useComplexes: () => complexesMock() as unknown }));
vi.mock('@/features/courts', () => ({ useCourts: () => courtsMock() as unknown }));
vi.mock('@/features/auth', () => ({ useLogout: () => vi.fn() }));
vi.mock('@/shared/stores', () => ({
  useStore: (selector: (s: { setSelectedComplexId: () => void }) => unknown) =>
    selector({ setSelectedComplexId: vi.fn() }),
}));

const created = { id: 'c-new', mp_user_id: null } as unknown as Complex;

/** Renders the hook inside a router entered the way "Agregar complejo" enters it. */
function renderInNewComplexFlow() {
  let location: ReturnType<typeof useLocation> | null = null;

  function Probe() {
    location = useLocation();
    return null;
  }

  const wrapper = ({ children }: { children: ReactNode }) => (
    <MemoryRouter initialEntries={[{ pathname: '/onboarding', state: { newComplex: true } }]}>
      {children}
      <Probe />
    </MemoryRouter>
  );

  const rendered = renderHook(() => useOnboarding(), { wrapper });
  return { ...rendered, getLocation: () => location };
}

beforeEach(() => {
  vi.clearAllMocks();
  complexesMock.mockReturnValue({ data: [created], isLoading: false });
  courtsMock.mockReturnValue({ data: [], isLoading: false });
});

describe('useOnboarding — surviving a reload after the complex is created', () => {
  // The bug: `justCreatedId` is component state and a reload wipes it, while
  // `newComplex: true` lives in the history entry and does not. The owner came
  // back to step 1, filled the form again, and ended up with two complexes for
  // one venue. The created id has to travel in the history entry too.
  it('writes the created complex into history state, replacing the newComplex entry', () => {
    const { result, getLocation } = renderInNewComplexFlow();

    expect((getLocation()?.state as { newComplex?: boolean } | null)?.newComplex).toBe(true);

    act(() => {
      result.current.handleComplexCreated(created);
    });

    const state = getLocation()?.state as { complexId?: string; newComplex?: boolean } | null;
    expect(state?.complexId).toBe('c-new');
    expect(state?.newComplex).toBeUndefined();
  });

  it('resolves the complex from history state alone, the way a reload would', () => {
    // A fresh mount with only what the history entry carries — no component
    // state left over from the create — still knows which complex is being set up.
    const wrapper = ({ children }: { children: ReactNode }) => (
      <MemoryRouter initialEntries={[{ pathname: '/onboarding', state: { complexId: 'c-new' } }]}>
        {children}
      </MemoryRouter>
    );

    const { result } = renderHook(() => useOnboarding(), { wrapper });

    expect(result.current.complexId).toBe('c-new');
    expect(result.current.step).not.toBe(1);
  });
});
