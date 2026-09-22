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

vi.mock('@/shared/components/common/ComplexLoadError', () => ({
  ComplexLoadError: ({ onRetry }: { onRetry: () => void }) => (
    <div data-testid="complex-load-error">
      <button onClick={onRetry}>retry</button>
    </div>
  ),
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

vi.mock('react-router-dom', () => ({
  Outlet: () => <div data-testid="outlet">Outlet Content</div>,
  Navigate: ({ to }: { to: string }) => <div data-testid="navigate">{to}</div>,
  Link: ({ children, to }: { children: React.ReactNode; to: string }) => <a href={to}>{children}</a>,
}));

import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { makeComplex } from '@/test/factories';

// Trims each test's mock to only what it varies.
function mockSelectedComplex(overrides: Partial<ReturnType<typeof useSelectedComplex>>) {
  vi.mocked(useSelectedComplex).mockReturnValue({
    selectedComplexId: null,
    needsOnboarding: false,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    complex: null,
    complexes: [],
    ...overrides,
  });
}

describe('DashboardLayout', () => {
  beforeEach(() => {
    mockSelectedComplex({ selectedComplexId: 'complex-1', complex: makeComplex({ id: 'complex-1', name: 'Test' }) });
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
    mockSelectedComplex({ isLoading: true });
    const { DashboardLayout } = await import('./DashboardLayout');
    render(<DashboardLayout />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('redirects to /onboarding when needsOnboarding is true', async () => {
    mockSelectedComplex({ needsOnboarding: true });
    const { DashboardLayout } = await import('./DashboardLayout');
    render(<DashboardLayout />);
    expect(screen.getByTestId('navigate')).toHaveTextContent('/onboarding');
  });

  it('shows a retryable error instead of redirecting when the complexes query errored', async () => {
    const refetch = vi.fn();
    mockSelectedComplex({ isError: true, error: new Error('Network error'), refetch });
    const { DashboardLayout } = await import('./DashboardLayout');
    render(<DashboardLayout />);

    expect(screen.getByTestId('complex-load-error')).toBeInTheDocument();
    expect(screen.queryByTestId('navigate')).not.toBeInTheDocument();

    screen.getByRole('button', { name: 'retry' }).click();
    expect(refetch).toHaveBeenCalled();
  });

  it('keeps rendering the dashboard when a background refetch fails but a complex is still cached', async () => {
    const complex = makeComplex({ id: 'complex-1', name: 'Test' });
    mockSelectedComplex({
      selectedComplexId: 'complex-1',
      complex,
      complexes: [complex],
      isError: true,
      error: new Error('Network error'),
    });
    const { DashboardLayout } = await import('./DashboardLayout');
    render(<DashboardLayout />);

    expect(screen.queryByTestId('complex-load-error')).not.toBeInTheDocument();
    expect(screen.queryByTestId('navigate')).not.toBeInTheDocument();
    expect(screen.getAllByTestId('sidebar').length).toBeGreaterThanOrEqual(1);
  });
});
