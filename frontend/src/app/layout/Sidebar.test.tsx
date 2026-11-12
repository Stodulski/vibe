import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Sidebar } from './Sidebar';

const mockUser = { first_name: 'Juan', last_name: 'Garcia', email: 'juan@test.com' };

// DATA-11: the signed-in user is server state and comes from `useAuth`'s
// query cache now, not from the zustand store.
vi.mock('@/features/auth/hooks/useAuth', () => ({
  useAuth: () => ({ user: mockUser, isLoading: false, isAuthenticated: true }),
}));

vi.mock('@/features/auth/hooks/useLogout', () => ({
  useLogout: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({
  useSelectedComplex: () => ({
    complex: { id: 'c1', name: 'Club Padel' },
  }),
}));

vi.mock('./usePrefetch', () => ({
  usePrefetch: () => vi.fn(),
}));

vi.mock('@/shared/components/common/ConfirmDialog', () => ({
  ConfirmDialog: () => null,
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

function renderSidebar() {
  return render(
    <MemoryRouter>
      <Sidebar isMobile />
    </MemoryRouter>,
  );
}

describe('Sidebar', () => {
  // The sidebar never draws the brand, at either width. The shell's masthead
  // owns it and stays on screen while the drawer is open, so a logo in here
  // was the same mark twice, one above the other.
  it.each([
    ['desktop', false],
    ['mobile', true],
  ])('does not render a brand row (%s)', (_width, isMobile) => {
    render(
      <MemoryRouter>
        <Sidebar isMobile={isMobile} />
      </MemoryRouter>,
    );
    expect(screen.queryByAltText('Vibe')).not.toBeInTheDocument();
  });

  it('renders navigation items', () => {
    renderSidebar();
    expect(screen.getByText('Dashboard')).toBeInTheDocument();
    expect(screen.getByText('Reservas')).toBeInTheDocument();
    expect(screen.getByText('Canchas')).toBeInTheDocument();
    expect(screen.getByText('Clientes')).toBeInTheDocument();
    expect(screen.getByText('Reportes')).toBeInTheDocument();
  });

  it('renders settings nav item', () => {
    renderSidebar();
    expect(screen.getByText('Configuración')).toBeInTheDocument();
  });

  // Owner request: each account has one complex, so its name label was dropped.
  it('does not render the complex name', () => {
    renderSidebar();
    expect(screen.queryByText('Club Padel')).not.toBeInTheDocument();
  });

  // Owner request: the account entry reads "Mi cuenta" instead of the email.
  it('labels the account entry "Mi cuenta"', () => {
    renderSidebar();
    expect(screen.getByRole('button', { name: /Juan Garcia/ })).toHaveTextContent('Mi cuenta');
    expect(screen.queryByText('juan@test.com')).not.toBeInTheDocument();
  });

  it('has sidebar aria-label', () => {
    renderSidebar();
    expect(screen.getByLabelText('Barra lateral')).toBeInTheDocument();
  });

  it('renders navigation with correct aria-label', () => {
    renderSidebar();
    expect(screen.getByLabelText('Navegación principal')).toBeInTheDocument();
  });

  it('gives every nav link a non-empty accessible name', () => {
    renderSidebar();
    const nav = screen.getByLabelText('Navegación principal');
    const links = within(nav).getAllByRole('link');
    expect(links.length).toBeGreaterThan(0);
    for (const link of links) {
      expect(link).toHaveAccessibleName();
    }
  });
});

describe('Sidebar — labels and icons per width', () => {
  // Owner request: the desktop rail names every part and drops the icons to
  // save space, instead of an icon-only rail with tooltips.
  it('names every part on desktop and draws no icons', () => {
    render(
      <MemoryRouter>
        <Sidebar />
      </MemoryRouter>,
    );
    const nav = screen.getByLabelText('Navegación principal');
    for (const label of ['Dashboard', 'Reservas', 'Caja', 'Canchas', 'Clientes', 'Reportes', 'Configuración']) {
      expect(within(nav).getByRole('link', { name: label })).toHaveTextContent(label);
    }
    expect(nav.querySelector('svg')).toBeNull();
    // The account entry keeps a profile icon (owner request).
    expect(screen.getByRole('button', { name: /Juan Garcia/ }).querySelector('svg')).not.toBeNull();
    expect(screen.getByRole('button', { name: /Juan Garcia/ })).toHaveTextContent('Mi cuenta');
  });

  // Owner request: the mobile sheet drops the nav icons too.
  it('draws no nav icons in the mobile sheet', () => {
    renderSidebar();
    expect(screen.getByLabelText('Navegación principal').querySelector('svg')).toBeNull();
  });
});
