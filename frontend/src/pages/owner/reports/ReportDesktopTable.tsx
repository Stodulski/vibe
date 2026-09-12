import { formatPrice } from '@/shared/lib/utils';
import type { MonthlyReport, MonthlyReportMethod } from '@/shared/types/api.types';
import { METHOD_LABELS, MONTH_NAMES } from './constants';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;
function ReportTableHead() {
  return (
    <thead>
      <tr className="border-border-subtle border-b text-left">
        <th scope="col" className="text-text-tertiary pr-4 pb-3 text-xs font-semibold tracking-wider uppercase">
          {t.reports.method}
        </th>
        <th
          scope="col"
          className="text-text-tertiary pr-4 pb-3 text-right text-xs font-semibold tracking-wider uppercase"
        >
          {t.reports.count}
        </th>
        <th
          scope="col"
          className="text-text-tertiary pr-4 pb-3 text-right text-xs font-semibold tracking-wider uppercase"
        >
          {t.reports.total}
        </th>
        <th
          scope="col"
          className="text-text-tertiary pr-4 pb-3 text-right text-xs font-semibold tracking-wider uppercase"
        >
          {t.reports.refunded}
        </th>
        <th scope="col" className="text-text-tertiary pb-3 text-right text-xs font-semibold tracking-wider uppercase">
          {t.reports.net}
        </th>
      </tr>
    </thead>
  );
}

interface ReportTableBodyProps {
  methodEntries: [string, MonthlyReportMethod][];
  totals: MonthlyReport['totals'];
}

function ReportTableBody({ methodEntries, totals }: ReportTableBodyProps) {
  return (
    <tbody>
      {methodEntries.map(([method, data]) => (
        <tr key={method} className="border-border-subtle/50 border-b">
          <td className="text-text-primary py-3 pr-4">{METHOD_LABELS[method] ?? method}</td>
          <td className="text-text-secondary py-3 pr-4 text-right tabular-nums">{data.count}</td>
          <td className="text-text-secondary py-3 pr-4 text-right tabular-nums">{formatPrice(data.total)}</td>
          <td className="text-text-secondary py-3 pr-4 text-right tabular-nums">{formatPrice(data.refunded)}</td>
          <td className="text-text-primary py-3 text-right font-medium tabular-nums">{formatPrice(data.net)}</td>
        </tr>
      ))}

      {/* Totals row */}
      <tr className="border-border-subtle border-t">
        <td className="text-text-primary pt-3 pr-4 text-sm font-bold">{t.reports.total}</td>
        <td className="text-text-primary pt-3 pr-4 text-right font-bold tabular-nums">{totals.count}</td>
        <td className="text-text-primary pt-3 pr-4 text-right font-bold tabular-nums">{formatPrice(totals.total)}</td>
        <td className="text-text-primary pt-3 pr-4 text-right font-bold tabular-nums">
          {formatPrice(totals.refunded)}
        </td>
        <td className="text-text-primary pt-3 text-right font-bold tabular-nums">{formatPrice(totals.net)}</td>
      </tr>
    </tbody>
  );
}

interface ReportDesktopTableProps {
  month: number;
  year: number;
  methodEntries: [string, MonthlyReportMethod][];
  totals: MonthlyReport['totals'];
}

export function ReportDesktopTable({ month, year, methodEntries, totals }: ReportDesktopTableProps) {
  return (
    <div className="-mx-4 hidden overflow-x-auto px-4 sm:mx-0 sm:block sm:px-0">
      <table
        className="w-full min-w-[480px] text-sm"
        aria-label={`${t.reports.tableLabel} - ${MONTH_NAMES[month - 1] ?? ''} ${String(year)}`}
      >
        <ReportTableHead />
        <ReportTableBody methodEntries={methodEntries} totals={totals} />
      </table>

      {/* Service fees note */}
      {totals.service_fees > 0 && (
        <p className="text-text-tertiary mt-4 text-xs">
          {t.reports.serviceFees}: {formatPrice(totals.service_fees)}
        </p>
      )}
    </div>
  );
}
