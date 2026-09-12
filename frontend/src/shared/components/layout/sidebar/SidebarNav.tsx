import { useLocation } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { mainNavItems, secondaryNavItems } from './navItems';
import { SidebarNavLink } from './SidebarNavLink';

const t = ES_AR;

interface SidebarNavProps {
  collapsed: boolean;
  isMobile: boolean;
  onNavigate?: (() => void) | undefined;
  onPrefetch: (to: string) => () => void;
}

export function SidebarNav({ collapsed, isMobile, onNavigate, onPrefetch }: SidebarNavProps) {
  const location = useLocation();

  const isItemActive = (to: string) =>
    to === '/settings' ? location.pathname.startsWith('/settings') : location.pathname === to;

  return (
    <nav className="flex-1 overflow-x-hidden overflow-y-auto px-3 pt-1" aria-label={t.layout.mainNav}>
      <div className="space-y-0.5">
        {mainNavItems.map((item) => (
          <SidebarNavLink
            key={item.to}
            item={item}
            isActive={isItemActive(item.to)}
            collapsed={collapsed}
            isMobile={isMobile}
            onNavigate={onNavigate}
            onMouseEnter={onPrefetch(item.to)}
          />
        ))}
      </div>

      <div className={cn('mt-4', collapsed && !isMobile && 'mt-3')}>
        <div className={cn('mb-2', collapsed && !isMobile ? 'mx-2' : 'mx-2.5', 'border-border-subtle/60 border-t')} />
        <div className="space-y-0.5">
          {secondaryNavItems.map((item) => (
            <SidebarNavLink
              key={item.to}
              item={item}
              isActive={isItemActive(item.to)}
              collapsed={collapsed}
              isMobile={isMobile}
              onNavigate={onNavigate}
              onMouseEnter={onPrefetch(item.to)}
            />
          ))}
        </div>
      </div>
    </nav>
  );
}
