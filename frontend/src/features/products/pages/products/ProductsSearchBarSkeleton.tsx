import { Skeleton } from '@/shared/components/ui/skeleton';
import { PRODUCTS_SEARCH_BAR_CLASS } from './productsPageLayout';

/** Loading stand-in for `ProductsSearchBar`: the search input row. */
export function ProductsSearchBarSkeleton() {
  return (
    <div className={PRODUCTS_SEARCH_BAR_CLASS}>
      <Skeleton className="h-11 w-full rounded-xl" />
    </div>
  );
}
