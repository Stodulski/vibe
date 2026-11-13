import { ES_AR } from '@/shared/i18n/es_AR';
import { SellCartSkeleton } from '../../components/sell/SellCartSkeleton';
import { SellProductGridSkeleton } from '../../components/sell/SellProductGridSkeleton';
import { SELL_LAYOUT_CLASS } from '../../components/sell/sellLayout';

const t = ES_AR;

/** Loading stand-in for the sell page body: catalog column plus the cart column from `lg`. */
export function SellOpenViewSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading} className={SELL_LAYOUT_CLASS}>
      <SellProductGridSkeleton />
      <SellCartSkeleton />
    </div>
  );
}
