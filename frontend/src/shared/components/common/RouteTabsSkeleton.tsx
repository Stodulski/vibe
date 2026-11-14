import { Skeleton } from '@/shared/components/ui/skeleton';
import { ROUTE_TAB_CLASS, ROUTE_TABS_NAV_CLASS, ROUTE_TABS_STRIP_CLASS } from './routeTabsLayout';

/** Loading stand-in for `RouteTabs`: `count` tabs. */
export function RouteTabsSkeleton({ count }: { count: number }) {
  return (
    <div className={ROUTE_TABS_NAV_CLASS}>
      <div className={ROUTE_TABS_STRIP_CLASS}>
        {Array.from({ length: count }, (_, i) => (
          <div key={i} className={ROUTE_TAB_CLASS}>
            <Skeleton className="h-4 w-14 rounded-lg" />
          </div>
        ))}
      </div>
    </div>
  );
}
