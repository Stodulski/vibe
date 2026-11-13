import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { SectionNavSkeleton } from '@/shared/components/common/section-nav/SectionNavSkeleton';
import { SectionTabsSkeleton } from '@/shared/components/common/section-nav/SectionTabsSkeleton';
import { SECTION_LAYOUT_CLASS, SECTION_PANEL_CLASS } from '@/shared/components/common/section-nav/sectionNavLayout';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { TABS } from './profile/tabs';

/**
 * What `ProfilePage` draws while its route chunk loads (the page has no data
 * wait of its own): mobile title, section tabs (below `lg`) or nav (from
 * `lg`), and the form block.
 */
export function ProfilePageSkeleton() {
  return (
    <div>
      <PageHeaderSkeleton />
      <div className={SECTION_LAYOUT_CLASS}>
        <SectionNavSkeleton count={TABS.length} />
        <SectionTabsSkeleton count={TABS.length} />
        <div className={`${SECTION_PANEL_CLASS} space-y-4`}>
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-16 w-full rounded-xl" />
          ))}
        </div>
      </div>
    </div>
  );
}
