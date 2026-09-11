import { useState, useMemo, useCallback, useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAdminComplexes } from '../../hooks/useAdminComplexes';
import { useIntersectionObserver } from '@/shared/hooks/useIntersectionObserver';

/** Reads the search term the URL carries; anything absent is an empty search. */
function readSearch(params: URLSearchParams): string {
  return params.get('search') ?? '';
}

/** Merges into the query, dropping keys set to null. Never pushes history. */
function useQueryPatch() {
  const [, setSearchParams] = useSearchParams();
  return useCallback(
    (patch: Record<string, string | null>) => {
      setSearchParams(
        (current) => {
          const next = new URLSearchParams(current);
          for (const [key, value] of Object.entries(patch)) {
            if (value === null) next.delete(key);
            else next.set(key, value);
          }
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );
}

/**
 * Table search, kept in the URL as `?search=`.
 *
 * Leaving for a row's detail page and coming back with the browser's back
 * button restores the search instead of losing it, and the filtered list is
 * shareable as a link. The debounce still runs over the local input value;
 * only the settled value reaches the URL.
 */
export function useComplexesTableState() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [initialSearch] = useState(() => readSearch(searchParams));
  const patchParams = useQueryPatch();

  const [searchInput, setSearchInput] = useState(initialSearch);
  const [debouncedSearch, setDebouncedSearch] = useState(initialSearch);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchInput);
      patchParams({ search: searchInput || null });
    }, 300);
    return () => {
      clearTimeout(timer);
    };
  }, [searchInput, patchParams]);

  const { data, isLoading, isError, refetch, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useAdminComplexes(debouncedSearch);
  const complexes = useMemo(() => data?.pages.flatMap((p) => p.complexes) ?? [], [data]);

  const sentinelRef = useIntersectionObserver(
    useCallback(() => {
      void fetchNextPage();
    }, [fetchNextPage]),
    hasNextPage && !isFetchingNextPage,
  );

  const handleRowClick = useCallback(
    (id: string) => {
      void navigate(`/admin/complexes/${id}`);
    },
    [navigate],
  );

  return {
    searchInput,
    setSearchInput,
    complexes,
    isLoading,
    isError,
    refetch,
    hasNextPage,
    isFetchingNextPage,
    sentinelRef,
    handleRowClick,
  };
}
