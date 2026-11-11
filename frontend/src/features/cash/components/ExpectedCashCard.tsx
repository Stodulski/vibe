import { Panel } from '@/shared/components/common/Panel';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn, formatPrice } from '@/shared/lib/utils';
import { formatVenueDayTime } from '@/shared/lib/formatVenueDayTime';
import { CashDifference } from './CashDifference';
import { cashMovementTotals } from '../lib/movementLedger';
import type { CashMovement, CashSession, CashSessionSummary } from '@/shared/types/api.types';

const t = ES_AR;

interface ExpectedCashCardProps {
  session: CashSession;
  summary: CashSessionSummary;
  /** The session's ledger, for the cash income and expense terms. */
  movements?: CashMovement[];
}

interface Term {
  label: string;
  amount: string;
  testId?: string;
}

/**
 * The terms that add up to expected cash, in the backend's order (opening +
 * cash income - cash expense + cash booking payments - cash refunds). Booking
 * payments and refunds only show when the session had any.
 */
function expectedCashTerms(session: CashSession, summary: CashSessionSummary, movements: CashMovement[]): Term[] {
  const { income, expense } = cashMovementTotals(movements);
  const cashBookingPayments = summary.booking_payments.find((p) => p.method === 'cash')?.amount ?? 0;
  const terms: Term[] = [
    { label: t.cash.openingCashLabel, amount: formatPrice(session.opening_cash) },
    { label: t.cash.incomeTotal, amount: formatPrice(income) },
    { label: t.cash.expenseTotal, amount: formatPrice(expense) },
  ];
  if (cashBookingPayments > 0) {
    terms.push({ label: t.cash.bookingPaymentsTitle, amount: formatPrice(cashBookingPayments) });
  }
  if (summary.cash_manual_refunds > 0) {
    terms.push({
      label: t.cash.cashManualRefunds,
      amount: `-${formatPrice(summary.cash_manual_refunds)}`,
      testId: 'cash-manual-refunds-row',
    });
  }
  return terms;
}

/**
 * The till at a glance: expected cash as the one large figure, and right
 * under it the terms it is made of, so the person at the counter can check
 * the number without scanning the page. For a closed session it also carries
 * the counted amount and the difference.
 */
export function ExpectedCashCard({ session, summary, movements = [] }: ExpectedCashCardProps) {
  const closed = !!session.closed_at;
  const terms = expectedCashTerms(session, summary, movements);

  return (
    // `data-testid`: the open-session summary and a closed session's history
    // row can show the same peso figures, and e2e needs to assert this
    // card's own values without matching a history row's (see cash.spec.ts).
    // Solid brand green, the till's key figure the way the dashboard's today
    // card is (owner decision, 2026-11-09); theme text tokens are re-pointed
    // to the near-black ink inside it.
    <Panel
      size="sm"
      mobile="card"
      className={cn(
        'border-primary-500 bg-primary-500 text-text-inverse w-full space-y-4',
        '[--color-text-primary:var(--color-text-inverse)]',
        '[--color-text-secondary:var(--color-text-inverse)]',
        '[--color-text-tertiary:var(--color-text-inverse)]',
        '[--color-border-subtle:rgb(11_11_11/0.2)]',
      )}
      data-testid="cash-expected-card"
    >
      <p className="text-sm">
        {t.cash.openedAt} {formatVenueDayTime(session.opened_at)}
        {closed && ` · ${t.cash.closedAt} ${formatVenueDayTime(session.closed_at)}`}
      </p>

      <div>
        <p className="text-sm font-semibold">{t.cash.expectedCash}</p>
        <p className="font-display text-3xl font-bold tracking-tight tabular-nums sm:text-4xl">
          {formatPrice(summary.expected_cash)}
        </p>
      </div>

      <dl className="border-border-subtle space-y-2 border-t pt-4">
        {terms.map((term) => (
          <div key={term.label} className="flex items-center justify-between gap-4 text-sm" data-testid={term.testId}>
            <dt>{term.label}</dt>
            <dd className="font-semibold tabular-nums">{term.amount}</dd>
          </div>
        ))}
      </dl>

      {closed && (
        <div className="border-border-subtle space-y-3 border-t pt-4">
          <div className="flex items-center justify-between text-sm">
            <span>{t.cash.countedCashLabel}</span>
            <span className="font-semibold tabular-nums">{formatPrice(summary.counted_cash ?? 0)}</span>
          </div>
          <CashDifference difference={summary.difference ?? 0} />
        </div>
      )}
    </Panel>
  );
}
