import { DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CashHintRow } from './CashHintRow';
import { CashDifference } from './CashDifference';
import type { CashSession } from '@/shared/types/api.types';

const t = ES_AR;

/** "After close, show the closed result and return to the closed state" (odd/tasks/pos-cashbox.md T3). */
export function ClosedSessionResult({ session, onDone }: { session: CashSession; onDone: () => void }) {
  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.cash.closeTitle}</DialogTitle>
      </DialogHeader>

      <div className="space-y-2">
        <CashHintRow label={t.cash.closeExpectedHint} value={session.expected_cash ?? 0} />
        <CashHintRow label={t.cash.countedCashLabel} value={session.counted_cash ?? 0} />
        <CashDifference difference={session.difference ?? 0} />
      </div>

      <SectionFooter onSubmit={onDone} submitLabel={t.common.close} />
    </>
  );
}
