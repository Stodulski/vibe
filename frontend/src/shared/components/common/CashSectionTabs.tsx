import { NavLink } from 'react-router-dom';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CASH_TAB_CLASS, CASH_TABS_NAV_CLASS, CASH_TABS_STRIP_CLASS } from './cashSectionTabsLayout';

const t = ES_AR;

const TABS = [
  { to: '/cash', label: t.cash.tabs.turno },
  { to: '/cash/sell', label: t.cash.tabs.vender },
  { to: '/cash/products', label: t.cash.tabs.productos },
] as const;

/**
 * The "Turno | Vender | Productos" tab strip shared by the three top-level
 * Caja screens (`odd/tasks/pos-cashbox.md` T5a). Lives in `shared/` because
 * `features/cash` and `features/products` both render it and features never
 * import from one another.
 *
 * Real `NavLink`s, not the `SectionTabs`/`SectionNav` value+onChange pattern
 * settings/profile use: those two live at one route with a `?tab=` query
 * param, while Turno/Vender/Productos are three distinct routes that must be
 * deep-linkable and support the browser's back button — an app-history push
 * per tab, not a query-param replace. `end` on `/cash` alone keeps its own
 * link from also matching `/cash/sell`, `/cash/products` or
 * `/cash/sessions/:id`/`/cash/products/:id` (deliberately not tabbed — those
 * render their own "Volver a ..." back link instead).
 */
export function CashSectionTabs() {
  return (
    // Bleeds exactly the layout's own gutter (px-3, sm:px-6 on <main>) so the
    // underline reaches the edges without widening the page on phones.
    <nav className={CASH_TABS_NAV_CLASS} aria-label={t.cash.title}>
      <div className={CASH_TABS_STRIP_CLASS}>
        {TABS.map((tab) => (
          <NavLink
            key={tab.to}
            to={tab.to}
            end={tab.to === '/cash'}
            className={({ isActive }) =>
              cn(
                CASH_TAB_CLASS,
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
