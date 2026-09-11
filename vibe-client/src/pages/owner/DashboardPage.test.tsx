import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));
vi.mock('@/shared/components/common/Skeletons', () => ({
  SkeletonDashboard: () => <div data-testid="skeleton-dashboard">Loading...</div>,
}));
vi.mock('@/shared/components/common/EmptyState', () => ({
  EmptyState: ({ title, actionLabel, onAction }: { title: string; actionLabel?: string; onAction?: () => void }) => (
    <div data-testid="empty-state">
      {title}
      {actionLabel && onAction && <button onClick={onAction}>{actionLabel}</button>}
    </div>
  ),
}));
vi.mock('./dashboard/PublicLinkBar', () => ({
  PublicLinkBar: () => <div data-testid="public-link-bar">PublicLinkBar</div>,
}));
vi.mock('./dashboard/DashboardContent', () => ({
  DashboardContent: () => <div data-testid="dashboard-content">DashboardContent</div>,
}));
vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({
  useSelectedComplex: vi.fn().mockReturnValue({ complex: null, selectedComplexId: 'c1' }),
}));
vi.mock('@/features/dashboard/hooks/useDashboardStats', () => ({
  useDashboardStats: vi.fn(),
}));
vi.mock('@/features/dashboard/hooks/useClientInsights', () => ({
  useClientInsights: vi.fn().mockReturnValue({ data: undefined, isLoading: false, isError: false, refetch: vi.fn() }),
}));

import { useDashboardStats } from '@/features/dashboard';

describe('DashboardPage', () => {
  it('shows loading skeleton while fetching', async () => {
    vi.mocked(useDashboardStats).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useDashboardStats>);
    const DashboardPage = (await import('./DashboardPage')).default;
    render(<DashboardPage />);
    expect(screen.getByTestId('skeleton-dashboard')).toBeInTheDocument();
  });

  it('shows dashboard content once stats load', async () => {
    vi.mocked(useDashboardStats).mockReturnValue({
      data: { upcoming_bookings: [] },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useDashboardStats>);
    const DashboardPage = (await import('./DashboardPage')).default;
    render(<DashboardPage />);
    expect(screen.getByTestId('dashboard-content')).toBeInTheDocument();
  });

  it('shows the error state with a retry action instead of the skeleton when the query fails', async () => {
    vi.mocked(useDashboardStats).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useDashboardStats>);
    const DashboardPage = (await import('./DashboardPage')).default;
    render(<DashboardPage />);
    expect(screen.getByTestId('empty-state')).toHaveTextContent('No pudimos cargar los datos.');
    expect(screen.queryByTestId('skeleton-dashboard')).not.toBeInTheDocument();
    expect(screen.queryByTestId('dashboard-content')).not.toBeInTheDocument();
  });

  it('calls refetch when the retry button is clicked', async () => {
    const refetch = vi.fn();
    vi.mocked(useDashboardStats).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      refetch,
    } as unknown as ReturnType<typeof useDashboardStats>);
    const DashboardPage = (await import('./DashboardPage')).default;
    render(<DashboardPage />);
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });
});
