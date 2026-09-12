import { formatPrice } from '@/shared/lib/utils';
import type { MonthlyReport, MonthlyReportMethod } from '@/shared/types/api.types';
import { METHOD_LABELS, MONTH_NAMES } from './constants';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;
function ReportTableHead() {
  return (
    <thead>
      <tr className="border-b border-border-subtle text-left">
        <th scope="col" className="pb-3 pr-4 text-xs font-semibold uppercase tracking-wider text-text-tertiary">
          {t.reports.method}
        </th>
        <th
          scope="col"
          className="pb-3 pr-4 text-right text-xs font-semibold uppercase tracking-wider text-text-tertiary"
        >
          {t.reports.count}
        </th>
        <th
          scope="col"
          className="pb-3 pr-4 text-right text-xs font-semibold uppercase tracking-wider text-text-tertiary"
        >
          {t.reports.total}
        </th>
        <th
          scope="col"
          className="pb-3 pr-4 text-right text-xs font-semibold uppercase tracking-wider text-text-tertiary"
        >
          {t.reports.refunded}
        </th>
        <th scope="col" className="pb-3 text-right text-xs font-semibold uppercase tracking-wider text-text-tertiary">
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
        <tr key={method} className="border-b border-border-subtle/50">
          <td className="py-3 pr-4 text-text-primary">{METHOD_LABELS[method] ?? method}</td>
          <td className="py-3 pr-4 text-right text-text-secondary tabular-nums">{data.count}</td>
          <td className="py-3 pr-4 text-right text-text-secondary tabular-nums">{formatPrice(data.total)}</td>
          <td className="py-3 pr-4 text-right text-text-secondary tabular-nums">{formatPrice(data.refunded)}</td>
          <td className="py-3 text-right text-text-primary tabular-nums font-medium">{formatPrice(data.net)}</td>
        </tr>
      ))}

      {/* Totals row */}
      <tr className="border-t border-border-subtle">
        <td className="pt-3 pr-4 text-sm font-bold text-text-primary">{t.reports.total}</td>
        <td className="pt-3 pr-4 text-right font-bold text-text-primary tabular-nums">{totals.count}</td>
        <td className="pt-3 pr-4 text-right font-bold text-text-primary tabular-nums">{formatPrice(totals.total)}</td>
        <td className="pt-3 pr-4 text-right font-bold text-text-primary tabular-nums">
          {formatPrice(totals.refunded)}
        </td>
        <td className="pt-3 text-right font-bold text-text-primary tabular-nums">{formatPrice(totals.net)}</td>
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
    <div className="hidden sm:block -mx-4 overflow-x-auto px-4 sm:mx-0 sm:px-0">
      <table
        className="w-full min-w-[480px] text-sm"
        aria-label={`${t.reports.tableLabel} - ${MONTH_NAMES[month - 1] ?? ''} ${String(year)}`}
      >
        <ReportTableHead />
        <ReportTableBody methodEntries={methodEntries} totals={totals} />
      </table>

      {/* Service fees note */}
      {totals.service_fees > 0 && (
        <p className="mt-4 text-xs text-text-tertiary">
          {t.reports.serviceFees}: {formatPrice(totals.service_fees)}
        </p>
      )}
    </div>
  );
}
