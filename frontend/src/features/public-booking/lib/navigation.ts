import { useCallback, useMemo, useSyncExternalStore } from 'react';

/**
 * Moves to another page with a full page load. The booking pages are not an
 * SPA route of their own: each step is its own document, so no router state
 * has to survive the move. `replace` leaves the current entry out of history.
 */
export function navigateTo(path: string, options?: { replace?: boolean }): void {
  if (options?.replace) {
    window.location.replace(path);
  } else {
    window.location.assign(path);
  }
}

export type SetQueryParams = (next: URLSearchParams | ((current: URLSearchParams) => URLSearchParams)) => void;

/**
 * Everyone who reads the query, so a write reaches every reader. Two hooks on
 * one page (the flow and its patch helper) must see the same URL, the way two
 * `useSearchParams` calls share one router state.
 */
const listeners = new Set<() => void>();

function subscribe(onChange: () => void): () => void {
  listeners.add(onChange);
  window.addEventListener('popstate', onChange);
  return () => {
    listeners.delete(onChange);
    window.removeEventListener('popstate', onChange);
  };
}

function readSearch(): string {
  return window.location.search;
}

/**
 * The query string of the current page, as a hook. Writes go through
 * `history.replaceState`, so they never add a history entry, and they keep the
 * path, the hash and the existing `history.state`. A `popstate` (Back or Forward)
 * re-reads the URL.
 */
export function useQueryParams(): [URLSearchParams, SetQueryParams] {
  const search = useSyncExternalStore(subscribe, readSearch, readSearch);
  const params = useMemo(() => new URLSearchParams(search), [search]);

  const setParams = useCallback<SetQueryParams>((next) => {
    const current = new URLSearchParams(window.location.search);
    const updated = typeof next === 'function' ? next(current) : next;
    const query = updated.toString();
    const url = `${window.location.pathname}${query ? `?${query}` : ''}${window.location.hash}`;
    window.history.replaceState(window.history.state, '', url);
    for (const listener of listeners) {
      listener();
    }
  }, []);

  return [params, setParams];
}
