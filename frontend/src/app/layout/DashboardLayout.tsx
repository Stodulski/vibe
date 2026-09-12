import { lazy, Suspense } from 'react';
import { Navigate } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { SidebarBrand } from '@/shared/components/layout/sidebar/SidebarBrand';
import { Header } from '@/shared/components/layout/Header';
import { AppShell } from '@/shared/components/layout/AppShell';
import { PageHeadingProvider } from '@/shared/components/layout/page-heading/PageHeadingProvider';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { useDashboardLayoutState } from './dashboard-layout/useDashboardLayoutState';
import { useNoIndex } from '@/shared/hooks/useNoIndex';

const CommandPalette = lazy(() =>
  import('@/shared/components/common/CommandPalette').then((m) => ({ default: m.CommandPalette })),
);

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

  // No complex at all -> go to complex selector
  if (state.needsOnboarding || !state.selectedComplexId) {
    return <Navigate to="/complexes" replace />;
  }

  return (
    <PageHeadingProvider>
      <AppShell
        brand={<SidebarBrand />}
        sidebar={<Sidebar />}
        renderMobileSidebar={(onNavigate) => <Sidebar onNavigate={onNavigate} isMobile />}
        header={(onMenuClick) => <Header onMenuClick={onMenuClick} />}
        showSearchTrigger
      >
        <Suspense>
          <CommandPalette />
        </Suspense>
      </AppShell>
    </PageHeadingProvider>
  );
}
