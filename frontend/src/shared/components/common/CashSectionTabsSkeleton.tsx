import { Skeleton } from '@/shared/components/ui/skeleton';
import { CASH_TAB_CLASS, CASH_TABS_NAV_CLASS, CASH_TABS_STRIP_CLASS } from './cashSectionTabsLayout';

/** Loading stand-in for `CashSectionTabs`. */
export function CashSectionTabsSkeleton() {
  return (
    <div className={CASH_TABS_NAV_CLASS}>
      <div className={CASH_TABS_STRIP_CLASS}>
        {[0, 1, 2].map((i) => (
          <div key={i} className={CASH_TAB_CLASS}>
            <Skeleton className="h-4 w-14 rounded-lg" />
          </div>
        ))}
      </div>
    </div>
  );
}
