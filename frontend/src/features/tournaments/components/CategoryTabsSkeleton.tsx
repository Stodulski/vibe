import { Skeleton } from '@/shared/components/ui/skeleton';
import { CATEGORY_TAB_CLASS, CATEGORY_TABS_STRIP_CLASS } from './categoryTabsLayout';

/** Loading shape of `CategoryTabs`: `count` underlined tabs of a typical name. */
export function CategoryTabsSkeleton({ count = 2 }: { count?: number }) {
  return (
    <div className={CATEGORY_TABS_STRIP_CLASS} aria-hidden="true">
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className={CATEGORY_TAB_CLASS}>
          <Skeleton className="h-4 w-20 rounded-md" />
        </div>
      ))}
    </div>
  );
}
