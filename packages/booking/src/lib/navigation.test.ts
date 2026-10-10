import { describe, it, expect, vi, afterEach } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { navigateTo, useQueryParams } from './navigation';

afterEach(() => {
  vi.restoreAllMocks();
  window.history.replaceState(null, '', '/');
});

describe('navigateTo', () => {
  it('assigns the path as a new history entry by default', () => {
    const assign = vi.spyOn(window.location, 'assign').mockImplementation(() => undefined);
    const replace = vi.spyOn(window.location, 'replace').mockImplementation(() => undefined);

    navigateTo('/c/club-norte/book/confirm');

    expect(assign).toHaveBeenCalledWith('/c/club-norte/book/confirm');
    expect(replace).not.toHaveBeenCalled();
  });

  it('replaces the current entry when asked to', () => {
    const assign = vi.spyOn(window.location, 'assign').mockImplementation(() => undefined);
    const replace = vi.spyOn(window.location, 'replace').mockImplementation(() => undefined);

    navigateTo('/c/club-norte', { replace: true });

    expect(replace).toHaveBeenCalledWith('/c/club-norte');
    expect(assign).not.toHaveBeenCalled();
  });
});

describe('useQueryParams', () => {
  it('reads the query of the current URL', () => {
    window.history.replaceState(null, '', '/c/club-norte?date=2026-03-18&duration=60');

    const { result } = renderHook(() => useQueryParams());

    expect(result.current[0].get('date')).toBe('2026-03-18');
    expect(result.current[0].get('duration')).toBe('60');
  });

  it('writes with replaceState: no new entry, and the router state is kept', () => {
    window.history.replaceState({ idx: 3 }, '', '/c/club-norte?date=2026-03-18');
    const lengthBefore = window.history.length;
    const { result } = renderHook(() => useQueryParams());

    act(() => {
      result.current[1]((current) => {
        const next = new URLSearchParams(current);
        next.set('time', '10:00');
        return next;
      });
    });

    expect(window.location.search).toBe('?date=2026-03-18&time=10%3A00');
    expect(window.history.length).toBe(lengthBefore);
    expect(window.history.state).toEqual({ idx: 3 });
    expect(result.current[0].get('time')).toBe('10:00');
  });

  it('keeps the hash when it writes', () => {
    window.history.replaceState(null, '', '/c/club-norte?date=2026-03-18#horarios');
    const { result } = renderHook(() => useQueryParams());

    act(() => {
      result.current[1](new URLSearchParams({ date: '2026-03-19' }));
    });

    expect(window.location.hash).toBe('#horarios');
    expect(window.location.search).toBe('?date=2026-03-19');
  });

  it('re-reads the query when popstate fires', () => {
    window.history.replaceState(null, '', '/c/club-norte?date=2026-03-18');
    const { result } = renderHook(() => useQueryParams());

    act(() => {
      window.history.replaceState(null, '', '/c/club-norte?date=2026-03-19');
      window.dispatchEvent(new PopStateEvent('popstate'));
    });

    expect(result.current[0].get('date')).toBe('2026-03-19');
  });
});
