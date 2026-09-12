import { ComparisonBadge } from '@/features/dashboard';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { MonthlyReportTotals } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * The month's net, with how it moved against the month before.
 *
 * The comparison lives here and nowhere else, on purpose. A delta beside every
 * figure — each method, each court — is a second number bolted onto every line
 * of a list that is already two columns wide at 320px, and the reader loses the
 * figures themselves in the badges. One number carries the direction; the
 * breakdowns below answer where it came from.
 *
 * The badge draws nothing when the previous month was zero or unchanged, which
 * is right: "up 100% from nothing" is not information, and a complex reporting
 * on its first month has no previous month at all.
 */
export function ReportHeadline({ totals, previous }: { totals: MonthlyReportTotals; previous: MonthlyReportTotals }) {
  return (
    <div className="border-border-subtle mb-5 border-b pb-4">
      <p className="text-micro text-text-tertiary font-medium tracking-wider uppercase">{t.reports.netForMonth}</p>
      <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1">
        <span className="score-text text-text-primary text-2xl font-bold tabular-nums">{formatPrice(totals.net)}</span>
        <ComparisonBadge current={totals.net} previous={previous.net} versus={t.dashboard.versusLastMonth} />
      </div>
      {previous.net > 0 && (
        <p className="score-text text-text-tertiary mt-1 text-xs tabular-nums">
          {t.reports.previousMonth}: {formatPrice(previous.net)}
        </p>
      )}
    </div>
  );
}
