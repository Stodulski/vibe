import { Navigate } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { SidebarBrand } from '@/shared/components/layout/sidebar/SidebarBrand';
import { Header } from '@/shared/components/layout/Header';
import { AppShell } from '@/shared/components/layout/AppShell';
import { PageHeadingProvider } from '@/shared/components/layout/page-heading/PageHeadingProvider';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { ComplexLoadError } from '@/shared/components/common/ComplexLoadError';
import { useDashboardLayoutState } from './dashboard-layout/useDashboardLayoutState';
import { useNoIndex } from '@/shared/hooks/useNoIndex';

export function DashboardLayout() {
  useNoIndex();
  const state = useDashboardLayoutState();

  if (state.isLoading) {
    return (
      <div className="bg-bg-base flex min-h-dvh items-center justify-center">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  // The complexes query failed rather than succeeding with an empty list —
  // redirecting to /onboarding here would 403 on submit ("the account
  // already owns a complex"). Only blanks the page when there is nothing
  // cached to fall back on: a background refetch failure (window focus,
  // reconnect) keeps `complexes` populated with the last good data, and the
  // dashboard should keep working through that instead of going blank.
  if (state.isError && state.complexes.length === 0) {
    return (
      <ComplexLoadError
        isFetching={state.isFetching}
        onRetry={() => {
          state.refetch();
        }}
      />
    );
  }

  // No complex at all -> go create one
  if (state.needsOnboarding || !state.selectedComplexId) {
    return <Navigate to="/onboarding" replace />;
  }

  return (
    <PageHeadingProvider>
      <AppShell
        brand={<SidebarBrand />}
        sidebar={<Sidebar />}
        renderMobileSidebar={(onNavigate) => <Sidebar onNavigate={onNavigate} isMobile />}
        header={(onMenuClick) => <Header onMenuClick={onMenuClick} />}
      />
    </PageHeadingProvider>
  );
}
