import { renderHook, act } from '@testing-library/react';
import { useSearchParams } from 'react-router-dom';
import { createWrapper } from '@/test/test-utils';
import { useUsersTableState } from './useUsersTableState';

const mockGetUsers = vi.fn();

vi.mock('../../api/admin.api', () => ({
  adminApi: {
    getUsers: (...args: unknown[]) => mockGetUsers(...args) as unknown,
  },
}));

function useProbe() {
  const state = useUsersTableState();
  const [params] = useSearchParams();
  return { state, params };
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  mockGetUsers.mockResolvedValue({
    users: [],
    metadata: { has_more: false, next_cursor: null },
  });
});

afterEach(() => {
  vi.useRealTimers();
});

// Finding M11: search and role filter used to live in useState, so leaving
// for a row's detail page and returning with the browser's back button lost
// both. They now live in the URL as `?search=&role=`.
describe('useUsersTableState — reading the URL on mount', () => {
  it('defaults to an empty search and "all roles" when the URL carries neither', () => {
    const { result } = renderHook(() => useProbe(), {
      wrapper: createWrapper(['/admin/users']),
    });

    expect(result.current.state.searchInput).toBe('');
    expect(result.current.state.roleFilter).toBe('_all');
  });

  it('restores search and role from the URL', () => {
    const { result } = renderHook(() => useProbe(), {
      wrapper: createWrapper(['/admin/users?search=juan&role=owner']),
    });

    expect(result.current.state.searchInput).toBe('juan');
    expect(result.current.state.roleFilter).toBe('owner');
  });

  it('falls back to "all roles" for a role the URL carries that is not a known filter', () => {
    const { result } = renderHook(() => useProbe(), {
      wrapper: createWrapper(['/admin/users?role=not-a-real-role']),
    });

    expect(result.current.state.roleFilter).toBe('_all');
  });
});

describe('useUsersTableState — writing to the URL', () => {
  it('writes the search term to the URL only after the debounce settles', async () => {
    const { result } = renderHook(() => useProbe(), {
      wrapper: createWrapper(['/admin/users']),
    });

    act(() => {
      result.current.state.setSearchInput('mar');
    });
    expect(result.current.params.get('search')).toBeNull();

    await act(() => vi.advanceTimersByTimeAsync(300));
    expect(result.current.params.get('search')).toBe('mar');
  });

  it('writes the role filter to the URL immediately, with no debounce', () => {
    const { result } = renderHook(() => useProbe(), {
      wrapper: createWrapper(['/admin/users']),
    });

    act(() => {
      result.current.state.setRoleFilter('superadmin');
    });
    expect(result.current.params.get('role')).toBe('superadmin');
  });

  it('drops the role param instead of writing "_all"', () => {
    const { result } = renderHook(() => useProbe(), {
      wrapper: createWrapper(['/admin/users?role=owner']),
    });

    act(() => {
      result.current.state.setRoleFilter('_all');
    });
    expect(result.current.params.get('role')).toBeNull();
  });
});
