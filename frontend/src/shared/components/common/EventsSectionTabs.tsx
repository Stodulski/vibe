import { ES_AR } from '@/shared/i18n/es_AR';
import { RouteTabs, type RouteTab } from './RouteTabs';

const TABS: readonly RouteTab[] = [
  { to: '/events/open-play', label: ES_AR.events.title },
  { to: '/events/tournaments', label: ES_AR.tournaments.title },
];

/**
 * The "Canchas libres | Torneos" tabs of the Eventos section. Lives in
 * `shared/` because `features/events` and `features/tournaments` both render it
 * and features never import from one another.
 */
export function EventsSectionTabs() {
  return <RouteTabs tabs={TABS} ariaLabel={ES_AR.navigation.events} />;
}
