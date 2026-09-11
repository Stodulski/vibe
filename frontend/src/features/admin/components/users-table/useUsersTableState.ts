import { useState, useMemo, useCallback, useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAdminUsers } from '../../hooks/useAdminUsers';
import { useIntersectionObserver } from '@/shared/hooks/useIntersectionObserver';
import { roleOptions } from './userTableUtils';

const DEFAULT_ROLE_FILTER = '_all';
const VALID_ROLE_FILTERS = new Set(roleOptions.map((option) => option.value));

/** Reads the search and role filter the URL carries; an unknown role falls back to "all". */
function readParams(params: URLSearchParams) {
  const role = params.get('role');
  return {
    search: params.get('search') ?? '',
    role: role && VALID_ROLE_FILTERS.has(role) ? role : DEFAULT_ROLE_FILTER,
  };
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
 * Table search and role filter, kept in the URL as `?search=&role=`.
 *
 * Leaving for a row's detail page and coming back with the browser's back
 * button restores both instead of losing them, and the filtered list is
 * shareable as a link. The debounce still runs over the local search value;
 * only the settled value reaches the URL. The role filter has no debounce
 * (a discrete select), so it is written as soon as it changes.
 */
export function useUsersTableState() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [initial] = useState(() => readParams(searchParams));
  const patchParams = useQueryPatch();

  const [searchInput, setSearchInput] = useState(initial.search);
  const [debouncedSearch, setDebouncedSearch] = useState(initial.search);
  const [roleFilter, setRoleFilterState] = useState(initial.role);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchInput);
      patchParams({ search: searchInput || null });
    }, 300);
    return () => {
      clearTimeout(timer);
    };
  }, [searchInput, patchParams]);

  const setRoleFilter = useCallback(
    (value: string) => {
      setRoleFilterState(value);
      patchParams({ role: value === DEFAULT_ROLE_FILTER ? null : value });
    },
    [patchParams],
  );

  const actualRole = roleFilter === DEFAULT_ROLE_FILTER ? '' : roleFilter;
  const { data, isLoading, isError, refetch, hasNextPage, fetchNextPage, isFetchingNextPage } = useAdminUsers(
    debouncedSearch,
    actualRole,
  );
  const users = useMemo(() => data?.pages.flatMap((p) => p.users) ?? [], [data]);

  const sentinelRef = useIntersectionObserver(
    useCallback(() => {
      void fetchNextPage();
    }, [fetchNextPage]),
    hasNextPage && !isFetchingNextPage,
  );

  const handleRowClick = useCallback(
    (id: string) => {
      void navigate(`/admin/users/${id}`);
    },
    [navigate],
  );

  return {
    searchInput,
    setSearchInput,
    roleFilter,
    setRoleFilter,
    users,
    isLoading,
    isError,
    refetch,
    hasNextPage,
    isFetchingNextPage,
    sentinelRef,
    handleRowClick,
  };
}
