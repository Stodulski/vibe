import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Sidebar } from './Sidebar';

const mockUser = { first_name: 'Juan', last_name: 'Garcia', email: 'juan@test.com' };

vi.mock('@/shared/stores', () => ({
  // Selector-aware, like the real zustand store (M1): `Sidebar` calls
  // `useStore((s) => s.user)` rather than reading the whole state.
  useStore: (selector: (s: { user: typeof mockUser }) => unknown) => selector({ user: mockUser }),
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

  it('renders complex name', () => {
    renderSidebar();
    expect(screen.getByText('Club Padel')).toBeInTheDocument();
  });

  it('renders the user email', () => {
    renderSidebar();
    expect(screen.getByText('juan@test.com')).toBeInTheDocument();
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
