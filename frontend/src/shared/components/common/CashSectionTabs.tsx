import { ES_AR } from '@/shared/i18n/es_AR';
import { RouteTabs, type RouteTab } from './RouteTabs';

const t = ES_AR;

const TABS: readonly RouteTab[] = [
  { to: '/cash', label: t.cash.tabs.turno, end: true },
  { to: '/cash/sell', label: t.cash.tabs.vender },
  { to: '/cash/products', label: t.cash.tabs.productos },
];

/**
 * The "Turno | Vender | Productos" tab strip shared by the three top-level
 * Caja screens (`odd/tasks/pos-cashbox.md` T5a). Lives in `shared/` because
 * `features/cash` and `features/products` both render it and features never
 * import from one another.
 *
 * Route tabs (see `RouteTabs`): Turno/Vender/Productos are three distinct
 * routes. `end` on `/cash` alone keeps its own
 * link from also matching `/cash/sell`, `/cash/products` or
 * `/cash/sessions/:id`/`/cash/products/:id` (deliberately not tabbed — those
 * render their own "Volver a ..." back link instead).
 */
export function CashSectionTabs() {
  return <RouteTabs tabs={TABS} ariaLabel={t.cash.title} />;
}
