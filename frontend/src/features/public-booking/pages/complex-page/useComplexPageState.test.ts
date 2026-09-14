import { describe, it, expect } from 'vitest';
import { createElement, type ReactNode } from 'react';
import { renderHook, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { format } from 'date-fns/format';
import type { SelectedSlot } from '@/features/public-booking';
import { hasUnsavedWork } from '@/shared/lib/unsavedWork';
import { useComplexPageState } from './useComplexPageState';

// Plain .ts file (no JSX loader here), so the router wrapper is built with
// createElement instead of JSX.
function wrapperFor(initialEntries: string[]) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return createElement(MemoryRouter, { initialEntries }, children);
  };
}

describe('useComplexPageState — reading a fully answered URL', () => {
  it('carries every answer from the query into state', () => {
    const { result } = renderHook(() => useComplexPageState(), {
      wrapper: wrapperFor(['/los-alamos?date=2099-01-05&duration=60&time=08:30&sport=padel']),
    });

    expect(result.current.duration).toBe(60);
    expect(result.current.selectedDuration).toBe(60);
    expect(result.current.sportFilter).toBe('padel');
    expect(result.current.pendingStartTime).toBe('08:30');
    expect(result.current.answeredFromUrl).toEqual({ sport: true, duration: true });
    expect(result.current.dateStr).toBe('2099-01-05');
  });
});

describe('useComplexPageState — reading a malformed URL', () => {
  it('falls back to the defaults for values it cannot make sense of', () => {
    const { result } = renderHook(() => useComplexPageState(), {
      wrapper: wrapperFor(['/los-alamos?duration=45&sport=chess&date=2020-01-01&time=99:99']),
    });

    // "99:99" has the shape of a time but is not one; a range check, not
    // just a shape check, keeps nonsense out of the crumb.
    expect(result.current.pendingStartTime).toBeNull();

    // duration=45 is not one of the offered durations, so it falls back to
    // the endpoint's own default (90) rather than an invented value.
    expect(result.current.duration).toBe(90);
    // But nothing was actually answered — the default is for the request,
    // not for what the picker should show as chosen (U-01).
    expect(result.current.selectedDuration).toBeNull();
    // 'chess' is not a Sport this app knows about.
    expect(result.current.sportFilter).toBeNull();
    expect(result.current.answeredFromUrl).toEqual({ sport: false, duration: false });
    // A past date is rejected; today is used instead.
    expect(result.current.dateStr).toBe(format(new Date(), 'yyyy-MM-dd'));
  });
});

describe('useComplexPageState — U-01: a fresh visit does not preselect a duration', () => {
  it('requests the default duration but reports nothing as chosen', () => {
    const { result } = renderHook(() => useComplexPageState(), {
      wrapper: wrapperFor(['/los-alamos']),
    });

    // The request default (90) keeps the initial availability fetch and the
    // picker's eventual options in agreement — that part stays.
    expect(result.current.duration).toBe(90);
    // But the visitor has not chosen anything yet, so nothing should render
    // as selected. Before this fix, `duration` (already defaulted to 90) was
    // the only value available, and got painted as the answer.
    expect(result.current.selectedDuration).toBeNull();

    act(() => {
      result.current.handleDurationChange(90);
    });
    // Explicitly choosing 90 — even the same number the default already
    // requested — is what makes it the selected value from here on.
    expect(result.current.selectedDuration).toBe(90);
  });
});

describe('useComplexPageState — changing answers after mount', () => {
  it('clears the pending start time when the duration changes, and reflects a newly set time', () => {
    const { result } = renderHook(() => useComplexPageState(), {
      wrapper: wrapperFor(['/los-alamos?time=08:30']),
    });
    expect(result.current.pendingStartTime).toBe('08:30');

    act(() => {
      result.current.handleDurationChange(120);
    });
    expect(result.current.duration).toBe(120);
    expect(result.current.pendingStartTime).toBeNull();

    act(() => {
      result.current.setPendingStartTime('20:00');
    });
    expect(result.current.pendingStartTime).toBe('20:00');
  });
});

// PWA-09: the PWA applies a waiting build when the tab goes to the background,
// and a client who picks a court, switches to WhatsApp and comes back must not
// find the page reset. Only the in-memory selection counts: the day, sport,
// duration and open hour are in the query and survive a reload by design.
describe('useComplexPageState — unsaved work while a slot is selected', () => {
  const slot = {
    courtId: 'ct1',
    courtName: 'Cancha 1',
    endTime: '11:30',
    durationMinutes: 90,
    totalPrice: 1500000,
    slot: { start_time: '10:00' },
  } as unknown as SelectedSlot;

  it('reports no unsaved work on a fresh visit', () => {
    renderHook(() => useComplexPageState(), { wrapper: wrapperFor(['/los-alamos']) });

    expect(hasUnsavedWork()).toBe(false);
  });

  it('marks the page busy once a slot is picked and clears it when the selection goes', () => {
    const { result } = renderHook(() => useComplexPageState(), {
      wrapper: wrapperFor(['/los-alamos']),
    });

    act(() => {
      result.current.setSelectedSlot(slot);
    });
    expect(hasUnsavedWork()).toBe(true);

    act(() => {
      result.current.setSelectedSlot(null);
    });
    expect(hasUnsavedWork()).toBe(false);
  });

  // Changing the day, the sport or the duration drops the selection — the
  // grid it belonged to is gone — so the page stops being busy with it.
  it('clears the registration when a flow answer drops the selection', () => {
    const { result } = renderHook(() => useComplexPageState(), {
      wrapper: wrapperFor(['/los-alamos']),
    });

    act(() => {
      result.current.setSelectedSlot(slot);
    });
    expect(hasUnsavedWork()).toBe(true);

    act(() => {
      result.current.handleDurationChange(120);
    });
    expect(hasUnsavedWork()).toBe(false);
  });

  // An answer the URL carries is not work a reload can lose.
  it('does not mark the page busy for a fully answered query alone', () => {
    renderHook(() => useComplexPageState(), {
      wrapper: wrapperFor(['/los-alamos?date=2099-01-05&duration=60&time=08:30&sport=padel']),
    });

    expect(hasUnsavedWork()).toBe(false);
  });

  it('clears the registration when the page unmounts with a slot still selected', () => {
    const { result, unmount } = renderHook(() => useComplexPageState(), {
      wrapper: wrapperFor(['/los-alamos']),
    });

    act(() => {
      result.current.setSelectedSlot(slot);
    });
    expect(hasUnsavedWork()).toBe(true);

    unmount();

    expect(hasUnsavedWork()).toBe(false);
  });
});
