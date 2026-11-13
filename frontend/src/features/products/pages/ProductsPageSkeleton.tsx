import { CashSectionTabsSkeleton } from '@/shared/components/common/CashSectionTabsSkeleton';
import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ProductGridSkeleton } from '../components/ProductGridSkeleton';
import { ProductsSearchBarSkeleton } from './products/ProductsSearchBarSkeleton';
import { PRODUCTS_BODY_CLASS } from './products/productsPageLayout';

/** Cold-load fallback for `/cash/products`: title + create button, tabs, search row, catalog. */
export function ProductsPageSkeleton() {
  return (
    <div>
      <PageHeaderSkeleton
        titleWidth="w-32"
        actions={<Skeleton className="h-10 w-full rounded-xl sm:w-40 md:ml-auto" />}
      />
      <CashSectionTabsSkeleton />
      <div className={PRODUCTS_BODY_CLASS}>
        <ProductsSearchBarSkeleton />
        <ProductGridSkeleton />
      </div>
    </div>
  );
}
