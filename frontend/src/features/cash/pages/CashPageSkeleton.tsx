import { CashSectionTabsSkeleton } from '@/shared/components/common/CashSectionTabsSkeleton';
import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { CashSummarySkeleton } from '../components/CashSummarySkeleton';

/** Cold-load fallback for `/cash`: title, tabs, then the shift summary. */
export function CashPageSkeleton() {
  return (
    <div>
      <PageHeaderSkeleton />
      <CashSectionTabsSkeleton />
      <CashSummarySkeleton />
    </div>
  );
}
