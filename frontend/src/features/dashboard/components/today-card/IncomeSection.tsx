import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { DayMoneyTotals } from '@/shared/types/api.types';

const t = ES_AR;

function IncomeLine({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-text-secondary">{label}</span>
      <span className="score-text text-text-primary font-medium">{formatPrice(value)}</span>
    </div>
  );
}

/**
 * Today's total income, split by where it came from, plus today's expenses
 * on their own line — income and "what's left after expenses" are different
 * questions (see DayMoneyTotals.total_income's own doc comment), so this
 * never subtracts one from the other.
 *
 * No ComparisonBadge here: the old "Ingresos de hoy" figure was
 * bookings-only, so a vs-yesterday comparison against THIS total (bookings +
 * bar sales + other income) would be comparing two different quantities. The
 * backend does not carry yesterday's total_income, so it is left out rather
 * than shown against the wrong baseline.
 */
export function IncomeSection({ totals }: { totals: DayMoneyTotals }) {
  return (
    <div className="flex flex-col gap-2">
      <div>
        <h3 className="text-text-primary text-sm font-semibold">{t.dashboard.todayRevenue}</h3>
        <p className="score-text text-text-primary text-2xl font-bold">{formatPrice(totals.total_income)}</p>
      </div>
      <div className="space-y-1">
        <IncomeLine label={t.dashboard.todayIncomeBookings} value={totals.bookings} />
        <IncomeLine label={t.dashboard.todayIncomeBar} value={totals.bar_sales} />
        <IncomeLine label={t.dashboard.todayIncomeOther} value={totals.other_income} />
      </div>
      <div className="border-border-subtle flex items-center justify-between border-t pt-2 text-sm">
        <span className="text-text-secondary">{t.cash.expenseTotal}</span>
        <span className="score-text text-error-text font-medium">{formatPrice(totals.expenses)}</span>
      </div>
    </div>
  );
}
