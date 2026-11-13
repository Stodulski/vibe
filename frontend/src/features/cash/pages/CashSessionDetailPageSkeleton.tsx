import { BackLinkSkeleton } from '@/shared/components/common/BackLinkSkeleton';
import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { CashSummarySkeleton } from '../components/CashSummarySkeleton';
import { CASH_STACK_CLASS } from '../components/cashLayout';

/** Cold-load fallback for `/cash/sessions/:id`: back link, title, closed-session summary. */
export function CashSessionDetailPageSkeleton() {
  return (
    <div className={CASH_STACK_CLASS}>
      <BackLinkSkeleton />
      <PageHeaderSkeleton titleWidth="w-40" />
      <CashSummarySkeleton showActions={false} />
    </div>
  );
}
