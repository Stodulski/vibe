import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

vi.mock('./AdminSidebar', () => ({
  AdminSidebar: () => <div data-testid="admin-sidebar">AdminSidebar</div>,
}));

vi.mock('@/shared/components/layout/Header', () => ({
  Header: ({ onMenuClick }: { onMenuClick: () => void }) => (
    <button data-testid="admin-header" onClick={onMenuClick}>
      Header
    </button>
  ),
}));

vi.mock('@/shared/components/ui/sheet', () => ({
  Sheet: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SheetContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SheetTitle: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock('@/shared/components/common/VisuallyHidden', () => ({
  VisuallyHidden: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    Outlet: () => <div data-testid="outlet">Outlet Content</div>,
  };
});

describe('AdminLayout', () => {
  it('renders the admin sidebar and outlet', async () => {
    const { AdminLayout } = await import('./AdminLayout');
    render(
      <MemoryRouter>
        <AdminLayout />
      </MemoryRouter>,
    );
    expect(screen.getAllByTestId('admin-sidebar').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByTestId('outlet')).toBeInTheDocument();
  });

  it('renders skip navigation link', async () => {
    const { AdminLayout } = await import('./AdminLayout');
    render(
      <MemoryRouter>
        <AdminLayout />
      </MemoryRouter>,
    );
    expect(screen.getByText('Ir al contenido principal')).toBeInTheDocument();
  });

  it('renders admin header', async () => {
    const { AdminLayout } = await import('./AdminLayout');
    render(
      <MemoryRouter>
        <AdminLayout />
      </MemoryRouter>,
    );
    expect(screen.getByTestId('admin-header')).toBeInTheDocument();
  });

  it('has main content area with correct id', async () => {
    const { AdminLayout } = await import('./AdminLayout');
    render(
      <MemoryRouter>
        <AdminLayout />
      </MemoryRouter>,
    );
    const main = document.getElementById('main-content');
    expect(main).toBeInTheDocument();
  });
});
