import { Link } from 'react-router-dom';
import { ChevronRight } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice, cn } from '@/shared/lib/utils';
import { formatSessionInstant } from '../lib/formatCashInstant';
import { cashDifferenceLabel } from '../lib/cashDifferenceLabel';
import type { CashSession } from '@/shared/types/api.types';

const t = ES_AR;

export function CashSessionHistoryRow({ session }: { session: CashSession }) {
  const difference = session.difference ?? 0;
  const { label: diffLabel, colorClass: diffColorClass } = cashDifferenceLabel(difference);

  return (
    <Link
      to={`/cash/sessions/${session.id}`}
      className="border-border-subtle hover:bg-bg-elevated flex items-center justify-between gap-3 border-b px-1 py-3 last:border-b-0"
    >
      <div className="min-w-0 space-y-0.5">
        <p className="text-text-primary text-sm font-medium">
          {formatSessionInstant(session.opened_at)} — {formatSessionInstant(session.closed_at)}
        </p>
        <p className="text-text-tertiary text-xs">
          {t.cash.expectedCash}: {formatPrice(session.expected_cash ?? 0)} · {t.cash.countedCashLabel}:{' '}
          {formatPrice(session.counted_cash ?? 0)}
        </p>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <span className={cn('text-sm font-semibold whitespace-nowrap', diffColorClass)}>
          {diffLabel}
          {difference !== 0 && `: ${formatPrice(Math.abs(difference))}`}
        </span>
        <ChevronRight className="text-text-tertiary size-4" aria-hidden="true" />
      </div>
    </Link>
  );
}
