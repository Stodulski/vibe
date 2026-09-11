import { useStore } from '@/shared/stores';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { AdminBrand } from '@/shared/components/layout/admin-sidebar/AdminBrand';
import { AdminNav } from '@/shared/components/layout/admin-sidebar/AdminNav';
import { AdminUserMenu } from './admin-sidebar/AdminUserMenu';

const t = ES_AR;

interface AdminSidebarProps {
  onNavigate?: () => void;
  /** True when rendered inside the mobile nav sheet — shows labels and its own brand row. */
  isMobile?: boolean;
}

/** Desktop is permanently icon-only (no expand/collapse toggle); the mobile sheet stays fully expanded. */
export function AdminSidebar({ onNavigate, isMobile = false }: AdminSidebarProps) {
  const user = useStore((s) => s.user);
  const collapsed = !isMobile;

  return (
    <aside
      className={cn('flex h-full shrink-0 flex-col', isMobile ? 'w-[220px]' : 'w-[68px] border-r border-border-subtle')}
      aria-label={t.layout.sidebar}
    >
      {isMobile && (
        <div className="flex h-16 shrink-0 items-center px-5">
          <AdminBrand />
        </div>
      )}
      <AdminNav collapsed={collapsed} isMobile={isMobile} onNavigate={onNavigate} />

      <AdminUserMenu user={user} collapsed={collapsed} isMobile={isMobile} onNavigate={onNavigate} />
    </aside>
  );
}
