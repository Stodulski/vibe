import { useState, useEffect, useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useSelectedComplex } from '@/features/complex';
import { useClients, useClientActions } from '@/features/clients';
import { useIntersectionObserver } from '@/shared/hooks/useIntersectionObserver';

// The search term lives in the URL (`?q=`) rather than only in local state,
// so a refresh or the back button returns the owner to the search they were
// looking at instead of always resetting to an empty list.
function useClientsList(selectedComplexId: string | null) {
  const [searchParams, setSearchParams] = useSearchParams();
  const searchInput = searchParams.get('q') ?? '';
  const [debouncedSearch, setDebouncedSearch] = useState(searchInput);

  const setSearchInput = (value: string) => {
    setSearchParams(
      (current) => {
        const next = new URLSearchParams(current);
        if (value) next.set('q', value);
        else next.delete('q');
        return next;
      },
      { replace: true },
    );
  };

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchInput);
    }, 300);
    return () => {
      clearTimeout(timer);
    };
  }, [searchInput]);

  const { data, isLoading, isError, refetch, hasNextPage, fetchNextPage, isFetchingNextPage } = useClients(
    selectedComplexId,
    debouncedSearch,
  );

  // The `useCallback` stays: `useIntersectionObserver` lists `onIntersect` in
  // an effect's dependencies, and an effect's firing is not something to hand
  // to the React Compiler's inferred memoization.
  const sentinelRef = useIntersectionObserver(
    useCallback(() => {
      void fetchNextPage();
    }, [fetchNextPage]),
    hasNextPage && !isFetchingNextPage,
  );

  const clients = data?.pages.flatMap((p) => p.clients) ?? [];

  return {
    searchInput,
    setSearchInput,
    clients,
    isLoading,
    isError,
    refetch,
    hasNextPage,
    isFetchingNextPage,
    sentinelRef,
  };
}

export function useClientsPage() {
  const { selectedComplexId } = useSelectedComplex();
  const list = useClientsList(selectedComplexId);
  const actions = useClientActions(selectedComplexId);

  return { selectedComplexId, ...list, ...actions };
}
