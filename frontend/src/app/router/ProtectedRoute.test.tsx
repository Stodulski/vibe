import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import type { User, UserRole } from '@/shared/types/api.types';

const mockReturnVal = { user: null as User | null, isLoading: false, isAuthenticated: false };

vi.mock('@/features/auth/hooks/useAuth', () => ({
  useAuth: () => mockReturnVal,
}));

import { ProtectedRoute } from './ProtectedRoute';

const mockUser: User = {
  id: 'u1',
  email: 'test@test.com',
  first_name: 'Juan',
  last_name: 'Perez',
  role: 'owner',
  phone: '1155550000',
  is_active: true,
  email_verified: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

function setAuth(user: User | null, isLoading = false) {
  mockReturnVal.user = user;
  mockReturnVal.isLoading = isLoading;
  mockReturnVal.isAuthenticated = !!user;
}

function renderProtected(allowedRoles?: UserRole[], initialEntry = '/protected') {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route
          path="/protected"
          element={
            <ProtectedRoute allowedRoles={allowedRoles}>
              <div>Protected Content</div>
            </ProtectedRoute>
          }
        />
        <Route path="/login" element={<div>Login Page</div>} />
        <Route path="/" element={<div>Home Page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('ProtectedRoute', () => {
  it('renders children when user is authenticated', () => {
    setAuth(mockUser);
    renderProtected();
    expect(screen.getByText('Protected Content')).toBeInTheDocument();
  });

  it('redirects to login when user is not authenticated', () => {
    setAuth(null);
    renderProtected();
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    expect(screen.getByText('Login Page')).toBeInTheDocument();
  });

  it('shows loading spinner when auth is loading', () => {
    setAuth(null, true);
    renderProtected();
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    expect(screen.queryByText('Login Page')).not.toBeInTheDocument();
    // The loading state must be announced to screen readers, not a bare spinner icon.
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('renders children when user role is in allowedRoles', () => {
    setAuth(mockUser);
    renderProtected(['owner', 'superadmin']);
    expect(screen.getByText('Protected Content')).toBeInTheDocument();
  });

  it('redirects when user role is not in allowedRoles', () => {
    setAuth({ ...mockUser, role: 'client' });
    renderProtected(['superadmin']);
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    expect(screen.getByText('Home Page')).toBeInTheDocument();
  });

  it('renders children when no role restriction is set', () => {
    setAuth({ ...mockUser, role: 'client' });
    renderProtected();
    expect(screen.getByText('Protected Content')).toBeInTheDocument();
  });
});
