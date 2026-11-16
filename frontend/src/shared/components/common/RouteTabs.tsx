import { NavLink } from 'react-router-dom';
import { cn } from '@/shared/lib/utils';
import { ROUTE_TAB_CLASS, ROUTE_TABS_NAV_CLASS, ROUTE_TABS_STRIP_CLASS } from './routeTabsLayout';

export interface RouteTab {
  to: string;
  label: string;
  /** Match `to` exactly: keeps a root tab from also lighting up on its sub-routes. */
  end?: boolean;
}

/**
 * An underlined tab strip whose tabs are real routes (`NavLink`s): deep-linkable
 * and one history entry per tab, unlike the `SectionTabs`/`SectionNav`
 * value+onChange pattern settings/profile use for `?tab=`. Shared by the Caja
 * screens and the Eventos tabs.
 */
export function RouteTabs({ tabs, ariaLabel }: { tabs: readonly RouteTab[]; ariaLabel: string }) {
  return (
    // Bleeds exactly the layout's own gutter (px-3, sm:px-6 on <main>) so the
    // underline reaches the edges without widening the page on phones.
    <nav className={ROUTE_TABS_NAV_CLASS} aria-label={ariaLabel}>
      <div className={ROUTE_TABS_STRIP_CLASS}>
        {tabs.map((tab) => (
          <NavLink
            key={tab.to}
            to={tab.to}
            end={tab.end ?? false}
            className={({ isActive }) =>
              cn(
                ROUTE_TAB_CLASS,
                'border-b-2 text-sm font-medium transition-colors',
                isActive
                  ? 'border-primary-500 text-primary-400'
                  : 'text-text-secondary hover:text-text-primary border-transparent',
              )
            }
          >
            {tab.label}
          </NavLink>
        ))}
      </div>
    </nav>
  );
}
