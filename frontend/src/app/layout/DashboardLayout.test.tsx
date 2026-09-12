import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';

vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({
  useSelectedComplex: vi.fn().mockReturnValue({
    selectedComplexId: 'complex-1',
    needsOnboarding: false,
    isLoading: false,
    complex: { id: 'complex-1', name: 'Test Complex' },
  }),
}));

vi.mock('@/shared/hooks/useRealtimeEvents', () => ({
  useRealtimeEvents: vi.fn(),
}));

vi.mock('@/shared/tour/useDashboardTour', () => ({
  useTour: () => ({ showDismissedHint: false, dismissHint: vi.fn() }),
}));

vi.mock('@/shared/tour/TourReadyContext', () => ({
  TourReadyProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock('./Sidebar', () => ({
  Sidebar: () => <div data-testid="sidebar">Sidebar</div>,
}));

vi.mock('@/shared/components/layout/Header', () => ({
  Header: ({ onMenuClick }: { onMenuClick: () => void }) => (
    <button data-testid="header" onClick={onMenuClick}>
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

vi.mock('@/shared/components/common/LoadingSpinner', () => ({
  LoadingSpinner: () => <div role="status">Loading</div>,
}));

vi.mock('@/shared/components/ui/alert-dialog', () => ({
  AlertDialog: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDialogDescription: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDialogFooter: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDialogTitle: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDialogAction: ({ children }: { children: React.ReactNode }) => <button>{children}</button>,
}));

vi.mock('@/shared/components/common/CommandPalette', () => ({
  CommandPalette: () => null,
}));

vi.mock('react-router-dom', () => ({
  Outlet: () => <div data-testid="outlet">Outlet Content</div>,
  Navigate: ({ to }: { to: string }) => <div data-testid="navigate">{to}</div>,
  Link: ({ children, to }: { children: React.ReactNode; to: string }) => <a href={to}>{children}</a>,
}));

import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { makeComplex } from '@/test/factories';

describe('DashboardLayout', () => {
  beforeEach(() => {
    vi.mocked(useSelectedComplex).mockReturnValue({
      selectedComplexId: 'complex-1',
      needsOnboarding: false,
      isLoading: false,
      complex: makeComplex({ id: 'complex-1', name: 'Test' }),
      complexes: [],
    });
  });

  it('renders the sidebar and outlet when complex is selected', async () => {
    const { DashboardLayout } = await import('./DashboardLayout');
    render(<DashboardLayout />);
    expect(screen.getAllByTestId('sidebar').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByTestId('outlet')).toBeInTheDocument();
  });

  it('renders skip navigation link', async () => {
    const { DashboardLayout } = await import('./DashboardLayout');
    render(<DashboardLayout />);
    const skipLink = screen.getByText('Ir al contenido principal');
    expect(skipLink).toBeInTheDocument();
    expect(skipLink).toHaveAttribute('href', '#main-content');
  });

  it('shows loading spinner when isLoading is true', async () => {
    vi.mocked(useSelectedComplex).mockReturnValue({
      selectedComplexId: null,
      needsOnboarding: false,
      isLoading: true,
      complex: null,
      complexes: [],
    });
    const { DashboardLayout } = await import('./DashboardLayout');
    render(<DashboardLayout />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('redirects to /complexes when needsOnboarding is true', async () => {
    vi.mocked(useSelectedComplex).mockReturnValue({
      selectedComplexId: null,
      needsOnboarding: true,
      isLoading: false,
      complex: null,
      complexes: [],
    });
    const { DashboardLayout } = await import('./DashboardLayout');
    render(<DashboardLayout />);
    expect(screen.getByTestId('navigate')).toHaveTextContent('/complexes');
  });
});
