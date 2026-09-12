import { isValidElement } from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider, type RouteObject } from 'react-router-dom';
import type { User } from '@/shared/types/api.types';

const mockAuth = { user: null as User | null, isLoading: false, isAuthenticated: false };

vi.mock('@/features/auth/hooks/useAuth', () => ({
  useAuth: () => mockAuth,
}));

import { ownerStandaloneRoutes } from './ownerRoutes';
import { ProtectedRoute } from './ProtectedRoute';

const STANDALONE_PATHS = ['/complexes', '/onboarding', '/settings/mp/callback'];

function renderAt(path: string) {
  const router = createMemoryRouter([...ownerStandaloneRoutes, { path: '/login', element: <div>Login Page</div> }], {
    initialEntries: [path],
  });
  return render(<RouterProvider router={router} />);
}

describe('ownerStandaloneRoutes', () => {
  it('hangs the standalone pages off a single pathless guard', () => {
    // RTE-02: the guard used to be repeated inside each route's `element`,
    // which remounted the auth check on every navigation between the three.
    expect(ownerStandaloneRoutes).toHaveLength(1);
    const [guard] = ownerStandaloneRoutes as [RouteObject];
    expect(guard.path).toBeUndefined();
    expect(isValidElement(guard.element) && guard.element.type).toBe(ProtectedRoute);
    expect(guard.children?.map((child) => child.path)).toEqual(STANDALONE_PATHS);
    for (const child of guard.children ?? []) {
      expect(isValidElement(child.element) && child.element.type).not.toBe(ProtectedRoute);
    }
  });

  it.each(STANDALONE_PATHS)('still sends an anonymous visitor of %s to the login page', async (path) => {
    mockAuth.user = null;
    mockAuth.isAuthenticated = false;
    renderAt(path);
    expect(await screen.findByText('Login Page')).toBeInTheDocument();
  });
});
