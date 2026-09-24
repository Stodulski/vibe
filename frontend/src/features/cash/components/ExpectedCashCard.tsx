import { Panel } from '@/shared/components/common/Panel';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import { formatVenueDayTime } from '@/shared/lib/formatVenueDayTime';
import { CashDifference } from './CashDifference';
import type { CashSession, CashSessionSummary } from '@/shared/types/api.types';

const t = ES_AR;

interface ExpectedCashCardProps {
  session: CashSession;
  summary: CashSessionSummary;
}

/**
 * The session header: when it opened (and closed, once it has), the opening
 * float, and — the one figure the person at the counter actually needs,
 * prominent on purpose — the expected cash. For a closed session this also
 * carries the final counted amount and difference (`summary.counted_cash`/
 * `difference` are only ever present once closed).
 */
export function ExpectedCashCard({ session, summary }: ExpectedCashCardProps) {
  const closed = !!session.closed_at;

  return (
    // `data-testid`: the open-session summary and a closed session's history
    // row can show the same peso figures, and e2e needs to assert this
    // card's own expected-cash value without matching a history row's (see
    // cash.spec.ts).
    <Panel size="sm" className="space-y-3" data-testid="cash-expected-card">
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 text-sm">
        <span className="text-text-tertiary">
          {t.cash.openedAt}: {formatVenueDayTime(session.opened_at)}
        </span>
        {closed && (
          <span className="text-text-tertiary">
            {t.cash.closedAt}: {formatVenueDayTime(session.closed_at)}
          </span>
        )}
      </div>

      <div className="flex items-center justify-between text-sm">
        <span className="text-text-tertiary">{t.cash.openingCashLabel}</span>
        <span className="font-medium">{formatPrice(session.opening_cash)}</span>
      </div>

      {/* Only shown when the session actually handed cash back by hand — a
          session with no manual refund never renders this line. */}
      {summary.cash_manual_refunds > 0 && (
        <div className="flex items-center justify-between text-sm" data-testid="cash-manual-refunds-row">
          <span className="text-text-tertiary">{t.cash.cashManualRefunds}</span>
          <span className="font-medium">-{formatPrice(summary.cash_manual_refunds)}</span>
        </div>
      )}

      <div className="border-border-subtle flex flex-wrap items-center justify-between gap-x-3 gap-y-1 border-t pt-3">
        <span className="text-text-secondary text-sm font-semibold">{t.cash.expectedCash}</span>
        <span className="font-display text-text-primary text-xl font-bold tracking-tight tabular-nums sm:text-2xl">
          {formatPrice(summary.expected_cash)}
        </span>
      </div>

      {closed && (
        <>
          <div className="flex items-center justify-between text-sm">
            <span className="text-text-tertiary">{t.cash.countedCashLabel}</span>
            <span className="font-medium">{formatPrice(summary.counted_cash ?? 0)}</span>
          </div>
          <CashDifference difference={summary.difference ?? 0} />
        </>
      )}
    </Panel>
  );
}
