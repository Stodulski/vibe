import { Panel } from '@/shared/components/common/Panel';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { cn } from '@/shared/lib/utils';
import { SELL_CART_PANEL_CLASS } from './sellLayout';

/** Loading shape of `SellCart`. Below `lg` the cart lives in a bottom bar/sheet, so nothing shows. */
export function SellCartSkeleton() {
  return (
    <Panel className={cn('hidden lg:block', SELL_CART_PANEL_CLASS)}>
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
