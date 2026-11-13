import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { DashboardContentSkeleton } from './dashboard/DashboardContentSkeleton';
import { PublicLinkBarSkeleton } from './dashboard/PublicLinkBarSkeleton';

/** What `DashboardPage` draws while its route chunk loads, chrome included: the mobile title, the public-link bar, then the content. */
export function DashboardPageSkeleton() {
  return (
    <div>
      <PageHeaderSkeleton titleWidth="w-40" />
      <PublicLinkBarSkeleton />
      <DashboardContentSkeleton />
    </div>
  );
}
