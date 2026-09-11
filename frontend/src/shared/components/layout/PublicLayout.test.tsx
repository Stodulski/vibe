import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { PublicLayout } from './PublicLayout';

vi.mock('./AppHeader', () => ({
  AppHeader: ({ className }: { className?: string }) => (
    <header data-testid="app-header" className={className}>
      AppHeader
    </header>
  ),
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    Outlet: () => <div data-testid="outlet">Outlet Content</div>,
  };
});

describe('PublicLayout', () => {
  it('renders the AppHeader', () => {
    render(
      <MemoryRouter>
        <PublicLayout />
      </MemoryRouter>,
    );
    expect(screen.getByTestId('app-header')).toBeInTheDocument();
  });

  it('renders the Outlet', () => {
    render(
      <MemoryRouter>
        <PublicLayout />
      </MemoryRouter>,
    );
    expect(screen.getByTestId('outlet')).toBeInTheDocument();
  });

  it('renders main content area with correct role', () => {
    render(
      <MemoryRouter>
        <PublicLayout />
      </MemoryRouter>,
    );
    expect(screen.getByRole('main')).toBeInTheDocument();
  });

  it('renders footer with powered by text', () => {
    render(
      <MemoryRouter>
        <PublicLayout />
      </MemoryRouter>,
    );
    expect(screen.getByText('Powered by Vibe')).toBeInTheDocument();
  });

  it('has skip-to-content main content id', () => {
    render(
      <MemoryRouter>
        <PublicLayout />
      </MemoryRouter>,
    );
    const main = screen.getByRole('main');
    expect(main).toHaveAttribute('id', 'main-content');
  });

  it('renders a skip link to the main content, like the authenticated shell does', () => {
    render(
      <MemoryRouter>
        <PublicLayout />
      </MemoryRouter>,
    );
    const skipLink = screen.getByRole('link', { name: 'Ir al contenido principal' });
    expect(skipLink).toHaveAttribute('href', '#main-content');
  });
});
