import { CashSectionTabsSkeleton } from '@/shared/components/common/CashSectionTabsSkeleton';
import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { SellOpenViewSkeleton } from './sell/SellOpenViewSkeleton';

/** Cold-load fallback for `/cash/sell`: title, tabs, then the sell view. */
export function CashSellPageSkeleton() {
  return (
    <div>
      <PageHeaderSkeleton />
      <CashSectionTabsSkeleton />
      <SellOpenViewSkeleton />
    </div>
  );
}
