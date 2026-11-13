import { Skeleton } from '@/shared/components/ui/skeleton';
import { SECTION_NAV_CLASS, SECTION_NAV_ITEM_CLASS } from './sectionNavLayout';

/** Loading stand-in for `SectionNav`: `count` rows, desktop only like the real one. */
export function SectionNavSkeleton({ count }: { count: number }) {
  return (
    <nav className={SECTION_NAV_CLASS}>
      <div className="space-y-1">
        {Array.from({ length: count }, (_, i) => (
          <div key={i} className={SECTION_NAV_ITEM_CLASS}>
            <Skeleton className="size-4 rounded" />
            <Skeleton className="h-4 w-24 rounded-lg" />
          </div>
        ))}
      </div>
    </nav>
  );
}
