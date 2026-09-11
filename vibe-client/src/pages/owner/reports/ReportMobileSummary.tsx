import { formatPrice } from '@/shared/lib/utils';
import type { MonthlyReport, MonthlyReportMethod } from '@/shared/types/api.types';
import { METHOD_LABELS } from './constants';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;
interface ReportMobileSummaryProps {
  methodEntries: [string, MonthlyReportMethod][];
  totals: MonthlyReport['totals'];
}

export function ReportMobileSummary({ methodEntries, totals }: ReportMobileSummaryProps) {
  return (
    <div className="space-y-3 sm:hidden">
      {methodEntries.map(([method, data]) => (
        <div key={method} className="flex items-center justify-between border-b border-border-subtle/50 pb-3">
          <span className="text-sm text-text-primary">{METHOD_LABELS[method] ?? method}</span>
          <div className="text-right">
            <p className="text-sm font-medium text-text-primary tabular-nums">{formatPrice(data.net)}</p>
            <p className="text-xs text-text-tertiary tabular-nums">
              {data.count} {t.reports.payments}
            </p>
          </div>
        </div>
      ))}
      <div className="flex items-center justify-between pt-1">
        <span className="text-sm font-bold text-text-primary">{t.reports.total}</span>
        <div className="text-right">
          <p className="text-sm font-bold text-text-primary tabular-nums">{formatPrice(totals.net)}</p>
          <p className="text-xs text-text-tertiary tabular-nums">
            {totals.count} {t.reports.payments}
          </p>
        </div>
      </div>
      {totals.service_fees > 0 && (
        <p className="text-xs text-text-tertiary">
          {t.reports.serviceFees}: {formatPrice(totals.service_fees)}
        </p>
      )}
    </div>
  );
}
