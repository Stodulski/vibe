import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { MonthlyReportCourt } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * What each court took in the month, highest first.
 *
 * Payment method answers how the money arrived, which is an accounting fact.
 * This answers where it came from, which is the one an owner can act on: a
 * court earning a tenth of its neighbour is a schedule problem, a pricing
 * problem or a surface problem, and none of that is visible in "Transferencia".
 *
 * One list at every width rather than a table that collapses. The row is a
 * name and a figure, which fits 320px without help, and the desktop version of
 * a two-column list is the same two-column list.
 */
export function ReportByCourt({ courts }: { courts: MonthlyReportCourt[] }) {
  if (courts.length === 0) return null;

  return (
    <section className="border-border-subtle mt-6 border-t pt-4">
      <h3 className="text-micro text-text-tertiary mb-3 font-medium tracking-wider uppercase">{t.reports.byCourt}</h3>
      <dl className="space-y-2">
        {courts.map((court) => (
          <div key={court.court_id} className="flex items-baseline justify-between gap-3">
            <dt className="text-text-primary min-w-0 truncate text-sm">
              {court.court_name === '' ? t.reports.deletedCourt : court.court_name}
            </dt>
            <dd className="shrink-0 text-right">
              <p className="score-text text-text-primary text-sm font-medium tabular-nums">{formatPrice(court.net)}</p>
              <p className="text-text-tertiary text-xs tabular-nums">
                {court.count} {t.reports.payments}
              </p>
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
