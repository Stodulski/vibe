import { Panel } from '@/shared/components/common/Panel';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

const TILE_COUNT = 8;

/** Mirrors `SellProductGrid`: search box, category chips, then list rows below `sm` and a 3 / 4 column tile grid above. */
function CatalogSkeleton() {
  return (
    <div className="min-w-0 space-y-3">
      <Skeleton className="h-11 w-full rounded-xl" />
      <div className="flex gap-2 overflow-hidden">
        {[0, 1, 2, 3, 4].map((i) => (
          <Skeleton key={i} className="h-9 w-24 shrink-0 rounded-full" />
        ))}
      </div>
      <div className="grid grid-cols-1 gap-2 pt-3 sm:grid-cols-3 sm:gap-3 lg:grid-cols-4">
        {Array.from({ length: TILE_COUNT }, (_, i) => (
          <Skeleton key={i} className="h-[58px] rounded-2xl sm:h-24" />
        ))}
      </div>
    </div>
  );
}

/** The cart is a sticky second column at `lg`; below it the cart lives in a bottom bar/sheet, so nothing shows. */
function CartSkeleton() {
  return (
    <Panel className="hidden lg:sticky lg:top-20 lg:block lg:h-[calc(100dvh-15rem)] lg:min-h-[24rem]">
      <Skeleton className="mb-4 h-7 w-24 rounded-lg" />
      <div className="space-y-4">
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-center justify-between gap-3">
            <Skeleton className="h-5 w-32 rounded-lg" />
            <Skeleton className="h-8 w-24 rounded-lg" />
          </div>
        ))}
      </div>
    </Panel>
  );
}

/** Loading stand-in for the sell page: catalog column plus the cart column from `lg`. */
export function CashSellSkeleton() {
  return (
    <div
      role="status"
      aria-label={t.common.loading}
      className="grid grid-cols-1 items-start gap-8 lg:grid-cols-[1fr_360px]"
    >
      <CatalogSkeleton />
      <CartSkeleton />
    </div>
  );
}
