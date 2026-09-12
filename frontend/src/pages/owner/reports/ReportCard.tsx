import { FileSpreadsheet } from 'lucide-react';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { Panel } from '@/shared/components/common/Panel';
import type { MonthlyReport } from '@/shared/types/api.types';
import { MONTH_NAMES } from './constants';
import { ReportMobileSummary } from './ReportMobileSummary';
import { ReportDesktopTable } from './ReportDesktopTable';
import { ReportHeadline } from './ReportHeadline';
import { ReportByCourt } from './ReportByCourt';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;
interface ReportCardProps {
  month: number;
  year: number;
  isLoading: boolean;
  isError: boolean;
  report: MonthlyReport | undefined;
}

export function ReportCard({ month, year, isLoading, isError, report }: ReportCardProps) {
  const methodEntries = report ? Object.entries(report.by_method) : [];

  return (
    <Panel as="section" size="md">
      <h2 className="text-text-primary mb-1 text-sm font-semibold">{t.reports.monthlyTitle}</h2>
      <p className="text-text-tertiary mb-4 text-xs">
        {MONTH_NAMES[month - 1]} {year}
      </p>

      {isLoading ? (
        <div className="flex justify-center py-12">
          <LoadingSpinner size="md" />
        </div>
      ) : isError ? (
        <EmptyState
          icon={FileSpreadsheet}
          title={t.dashboard.reportLoadError}
          description={t.dashboard.reportLoadErrorDescription}
        />
      ) : !report || (methodEntries.length === 0 && report.totals.count === 0) ? (
        <EmptyState
          icon={FileSpreadsheet}
          title={t.dashboard.reportNoData}
          description={t.dashboard.reportNoDataDescription}
        />
      ) : (
        <>
          <ReportHeadline totals={report.totals} previous={report.previous_totals} />
          <ReportMobileSummary methodEntries={methodEntries} totals={report.totals} />
          <ReportDesktopTable month={month} year={year} methodEntries={methodEntries} totals={report.totals} />
          <ReportByCourt courts={report.by_court} />
        </>
      )}
    </Panel>
  );
}
