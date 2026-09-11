import { LayoutDashboard, Users, Building2 } from 'lucide-react';
import { useLocation } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SidebarNavLink } from '../sidebar/SidebarNavLink';

const t = ES_AR;

const adminNavItems = [
  { to: '/admin', label: t.admin.navigation.dashboard, icon: LayoutDashboard },
  { to: '/admin/users', label: t.admin.navigation.users, icon: Users },
  { to: '/admin/complexes', label: t.admin.navigation.complexes, icon: Building2 },
] as const;

interface AdminNavProps {
  collapsed: boolean;
  isMobile: boolean;
  onNavigate?: (() => void) | undefined;
}

export function AdminNav({ collapsed, isMobile, onNavigate }: AdminNavProps) {
  const location = useLocation();

  const isItemActive = (to: string) =>
    to === '/admin' ? location.pathname === '/admin' : location.pathname.startsWith(to);

  return (
    <nav className="flex-1 overflow-y-auto overflow-x-hidden px-3 pt-1" aria-label={t.layout.mainNav}>
      <div className="space-y-0.5">
        {adminNavItems.map((item) => (
          <SidebarNavLink
            key={item.to}
            item={item}
            isActive={isItemActive(item.to)}
            collapsed={collapsed}
            isMobile={isMobile}
            onNavigate={onNavigate}
          />
        ))}
      </div>
    </nav>
  );
}
