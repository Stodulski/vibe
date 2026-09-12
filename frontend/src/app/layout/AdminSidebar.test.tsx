import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminSidebar } from './AdminSidebar';

const mockUser = { first_name: 'Admin', last_name: 'User', email: 'admin@test.com' };

// DATA-11: the signed-in user is server state and comes from `useAuth`'s
// query cache now, not from the zustand store.
vi.mock('@/features/auth/hooks/useAuth', () => ({
  useAuth: () => ({ user: mockUser, isLoading: false, isAuthenticated: true }),
}));

vi.mock('@/features/auth/hooks/useLogout', () => ({
  useLogout: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock('@/shared/components/ui/dropdown-menu', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return {
    DropdownMenu: passthrough('div'),
    DropdownMenuContent: passthrough('div'),
    DropdownMenuItem: passthrough('button'),
    DropdownMenuSeparator: () => <hr />,
    DropdownMenuTrigger: passthrough('div'),
  };
});

vi.mock('@/shared/components/ui/tooltip', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return {
    Tooltip: passthrough('div'),
    TooltipTrigger: passthrough('div'),
    TooltipContent: passthrough('div'),
  };
});

describe('AdminSidebar', () => {
  // Desktop has no expand/collapse toggle — it's permanently icon-only, so
  // it doesn't render its own brand row (the shell's masthead covers that).
  it('is icon-only and has no brand row by default (desktop)', () => {
    render(
      <MemoryRouter>
        <AdminSidebar />
      </MemoryRouter>,
    );
    expect(screen.queryByText('Admin')).not.toBeInTheDocument();
  });

  it('renders admin navigation items', () => {
    render(
      <MemoryRouter>
        <AdminSidebar isMobile />
      </MemoryRouter>,
    );
    expect(screen.getByText('Panel admin')).toBeInTheDocument();
    expect(screen.getByText('Usuarios')).toBeInTheDocument();
    expect(screen.getByText('Complejos')).toBeInTheDocument();
  });

  it('renders Vibe Admin brand', () => {
    render(
      <MemoryRouter>
        <AdminSidebar isMobile />
      </MemoryRouter>,
    );
    expect(screen.getByText('Admin')).toBeInTheDocument();
  });

  it('renders the user email', () => {
    render(
      <MemoryRouter>
        <AdminSidebar isMobile />
      </MemoryRouter>,
    );
    expect(screen.getByText('admin@test.com')).toBeInTheDocument();
  });

  it('has sidebar aria-label', () => {
    render(
      <MemoryRouter>
        <AdminSidebar isMobile />
      </MemoryRouter>,
    );
    expect(screen.getByLabelText('Barra lateral')).toBeInTheDocument();
  });

  it('renders main navigation with aria-label', () => {
    render(
      <MemoryRouter>
        <AdminSidebar isMobile />
      </MemoryRouter>,
    );
    expect(screen.getByLabelText('Navegación principal')).toBeInTheDocument();
  });
});
