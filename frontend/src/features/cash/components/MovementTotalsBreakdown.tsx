import { Panel } from '@/shared/components/common/Panel';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { CashMovementTotal } from '@/shared/types/api.types';

const t = ES_AR;

type CounterMethod = CashMovementTotal['method'];

/** Sums a kind's buckets by method — the summary is one row per (method, kind, category); this collapses category away. */
function totalsByMethod(totals: CashMovementTotal[], kind: 'income' | 'expense'): [CounterMethod, number][] {
  const byMethod = new Map<CounterMethod, number>();
  for (const bucket of totals) {
    if (bucket.kind !== kind) continue;
    byMethod.set(bucket.method, (byMethod.get(bucket.method) ?? 0) + bucket.total);
  }
  return [...byMethod.entries()].sort((a, b) => b[1] - a[1]);
}

function MethodTotalsGroup({ title, entries }: { title: string; entries: [CounterMethod, number][] }) {
  if (entries.length === 0) return null;
  return (
    <div className="space-y-1.5">
      <p className="text-text-tertiary text-xs font-semibold tracking-wider uppercase">{title}</p>
      {entries.map(([method, total]) => (
        <div key={method} className="flex items-center justify-between text-sm">
          <span className="text-text-secondary">{t.bookings.paymentMethods[method]}</span>
          <span className="font-medium">{formatPrice(total)}</span>
        </div>
      ))}
    </div>
  );
}

/** Cash-till income/expense totals per method (task: "totales: ingresos y egresos por método"). */
export function MovementTotalsBreakdown({ movementTotals }: { movementTotals: CashMovementTotal[] }) {
  const income = totalsByMethod(movementTotals, 'income');
  const expense = totalsByMethod(movementTotals, 'expense');
  if (income.length === 0 && expense.length === 0) return null;

  return (
    <Panel size="sm" className="space-y-3">
      <MethodTotalsGroup title={t.cash.incomeTotal} entries={income} />
      <MethodTotalsGroup title={t.cash.expenseTotal} entries={expense} />
    </Panel>
  );
}
