import { BackLinkSkeleton } from '@/shared/components/common/BackLinkSkeleton';
import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { ProductDetailBodySkeleton } from '../components/ProductDetailBodySkeleton';
import { PRODUCT_DETAIL_PAGE_CLASS } from '../components/productLayout';

/** Cold-load fallback for `/cash/products/:id`: back link, title, detail. */
export function ProductDetailPageSkeleton() {
  return (
    <div className={PRODUCT_DETAIL_PAGE_CLASS}>
      <BackLinkSkeleton />
      <PageHeaderSkeleton titleWidth="w-40" />
      <ProductDetailBodySkeleton />
    </div>
  );
}
