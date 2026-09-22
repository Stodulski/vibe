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

const created = { id: 'c-new', mp_user_id: null, created_at: '2026-09-14T10:00:00Z' } as unknown as Complex;
const existing = { id: 'c-existing', mp_user_id: null, created_at: '2026-09-14T12:00:00Z' } as unknown as Complex;

beforeEach(() => {
  vi.clearAllMocks();
  complexesMock.mockReturnValue({ data: [], isLoading: false });
  courtsMock.mockReturnValue({ data: [], isLoading: false });
});

describe('useOnboarding — surviving a reload after the complex is created', () => {
  // The bug: `justCreatedId` is component state and a reload wipes it. The
  // created id has to travel in the history entry too.
  it('writes the created complex into history state', () => {
    let location: ReturnType<typeof useLocation> | null = null;

    function Probe() {
      location = useLocation();
      return null;
    }

    const wrapper = ({ children }: { children: ReactNode }) => (
      <MemoryRouter initialEntries={['/onboarding']}>
        {children}
        <Probe />
      </MemoryRouter>
    );

    const { result } = renderHook(() => useOnboarding(), { wrapper });

    act(() => {
      result.current.handleComplexCreated(created);
    });

    const state = location as unknown as { state?: { complexId?: string } } | null;
    expect(state?.state?.complexId).toBe('c-new');
  });

  it('resolves the complex from history state alone, the way a reload would', () => {
    // A fresh mount with only what the history entry carries — no component
    // state left over from the create, and the complexes query hasn't caught
    // up yet either — still knows which complex is being set up.
    const wrapper = ({ children }: { children: ReactNode }) => (
      <MemoryRouter initialEntries={[{ pathname: '/onboarding', state: { complexId: 'c-new' } }]}>
        {children}
      </MemoryRouter>
    );

    const { result } = renderHook(() => useOnboarding(), { wrapper });

    expect(result.current.complexId).toBe('c-new');
    expect(result.current.step).not.toBe(1);
  });

  it('resolves the account single complex once the complexes query has it, with nothing in history', () => {
    complexesMock.mockReturnValue({ data: [existing], isLoading: false });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <MemoryRouter initialEntries={['/onboarding']}>{children}</MemoryRouter>
    );

    const { result } = renderHook(() => useOnboarding(), { wrapper });

    expect(result.current.complexId).toBe('c-existing');
  });
});
