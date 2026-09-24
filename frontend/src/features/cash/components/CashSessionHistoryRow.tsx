import { Link } from 'react-router-dom';
import { ChevronRight } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice, cn } from '@/shared/lib/utils';
import { cashDifferenceLabel } from '../lib/cashDifferenceLabel';
import { formatSessionRange } from '../lib/formatSessionRange';
import type { CashSession } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * One closed session in the history list. Two lines instead of two columns:
 * the span and the difference share the first line (the difference wraps
 * under the span rather than squeezing it), and expected/counted sit in a
 * two-column grid below, so no label or amount is split mid-phrase at 320 px.
 */
export function CashSessionHistoryRow({ session }: { session: CashSession }) {
  const difference = session.difference ?? 0;
  const { label: diffLabel, colorClass: diffColorClass } = cashDifferenceLabel(difference);

  return (
    <Link
      to={`/cash/sessions/${session.id}`}
      className="border-border-subtle hover:bg-bg-elevated flex items-center gap-2 border-b px-1 py-3 last:border-b-0"
    >
      <div className="min-w-0 flex-1 space-y-1.5">
        <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
          <p className="text-text-primary text-sm font-medium whitespace-nowrap tabular-nums">
            {formatSessionRange(session.opened_at, session.closed_at)}
          </p>
          <span className={cn('text-sm font-semibold whitespace-nowrap tabular-nums', diffColorClass)}>
            {diffLabel}
            {difference !== 0 && `: ${formatPrice(Math.abs(difference))}`}
          </span>
        </div>
        <dl className="grid grid-cols-2 gap-x-3 text-xs sm:flex sm:gap-x-8">
          <div className="min-w-0">
            <dt className="text-text-tertiary truncate">{t.cash.expectedCash}</dt>
            <dd className="text-text-secondary tabular-nums">{formatPrice(session.expected_cash ?? 0)}</dd>
          </div>
          <div className="min-w-0">
            <dt className="text-text-tertiary truncate">{t.cash.countedCashLabel}</dt>
            <dd className="text-text-secondary tabular-nums">{formatPrice(session.counted_cash ?? 0)}</dd>
          </div>
        </dl>
      </div>
      <ChevronRight className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
    </Link>
  );
}
