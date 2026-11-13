import { Skeleton } from '@/shared/components/ui/skeleton';
import { SECTION_TAB_CLASS, SECTION_TABS_CLASS } from './sectionNavLayout';

/** Loading stand-in for `SectionTabs`: `count` tabs, below `lg` only like the real one. */
export function SectionTabsSkeleton({ count }: { count: number }) {
  return (
    <div className={SECTION_TABS_CLASS}>
      <div className="flex">
        {Array.from({ length: count }, (_, i) => (
          <div key={i} className={`${SECTION_TAB_CLASS} border-transparent`}>
            <Skeleton className="h-4 w-16 rounded-lg" />
          </div>
        ))}
      </div>
    </div>
  );
}
