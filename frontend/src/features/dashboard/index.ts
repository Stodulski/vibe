// Public API of the dashboard feature.
// Only export what other features or pages actually consume from outside
// this folder — see 06-auth-shared-tooling.md A3.

export { TodayBookings } from './components/TodayBookings';
export { ComparisonBadge } from './components/stats-cards/ComparisonBadge';

export { useDashboardStats } from './hooks/useDashboardStats';
export { useClientInsights } from './hooks/useClientInsights';
export { useMonthlyReport } from './hooks/useMonthlyReport';

export { dashboardApi } from './api/dashboard.api';
