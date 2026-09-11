import { AdminSidebar } from './AdminSidebar';
import { AdminBrand } from '@/shared/components/layout/admin-sidebar/AdminBrand';
import { Header } from '@/shared/components/layout/Header';
import { AppShell } from '@/shared/components/layout/AppShell';
import { PageHeadingProvider } from '@/shared/components/layout/page-heading/PageHeadingProvider';
import { useNoIndex } from '@/shared/hooks/useNoIndex';

export function AdminLayout() {
  useNoIndex();

  return (
    <PageHeadingProvider>
      <AppShell
        brand={<AdminBrand collapsed />}
        sidebar={<AdminSidebar />}
        renderMobileSidebar={(onNavigate) => <AdminSidebar onNavigate={onNavigate} isMobile />}
        header={(onMenuClick) => <Header onMenuClick={onMenuClick} to="/admin" label="Admin" />}
      />
    </PageHeadingProvider>
  );
}
