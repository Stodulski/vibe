import { useLocation } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';
import { mainNavItems, secondaryNavItems } from './navItems';
import { SidebarNavLink } from './SidebarNavLink';

const t = ES_AR;

interface SidebarNavProps {
  collapsed: boolean;
  isMobile: boolean;
  onNavigate?: (() => void) | undefined;
  onPrefetch: (to: string) => () => void;
}

// Nav items whose section owns more than one route: `/settings` also covers
// its own sub-pages, and `/cash` covers Vender/Productos plus the read-only
// session/product detail routes (T5a: tabs and detail pages under `/cash`
// with no new sidebar item — "Caja" stays the active nav item on all of
// them). Every other item matches its own path exactly.
const PREFIX_MATCHED_ROOTS = ['/settings', '/cash'];

export function SidebarNav({ collapsed, isMobile, onNavigate, onPrefetch }: SidebarNavProps) {
  const location = useLocation();

  // A plain `startsWith` would also match an unrelated sibling route that
  // merely shares the prefix as text (`/cashflow` starting with `/cash`) — a
  // segment boundary keeps the match to `to` itself or one of its `/`-rooted
  // sub-paths.
  const isItemActive = (to: string) =>
    PREFIX_MATCHED_ROOTS.includes(to)
      ? location.pathname === to || location.pathname.startsWith(`${to}/`)
      : location.pathname === to;

  return (
    <nav className="flex-1 overflow-x-hidden overflow-y-auto px-3 pt-1" aria-label={t.layout.mainNav}>
      <div className="space-y-0.5">
        {[...mainNavItems, ...secondaryNavItems].map((item) => (
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
    </nav>
  );
}
