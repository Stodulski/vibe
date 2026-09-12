import { FileSpreadsheet } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { Panel } from '@/shared/components/common/Panel';
import { MONTH_NAMES } from './constants';
import { ReportMobileSummary } from './ReportMobileSummary';
import { ReportDesktopTable } from './ReportDesktopTable';
import { ReportHeadline } from './ReportHeadline';
import { ReportByCourt } from './ReportByCourt';
import { ReportCardSkeleton } from './ReportCardSkeleton';
import type { ReportCardState } from './reportCardState';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ReportCardProps {
  month: number;
  year: number;
  state: ReportCardState;
}

export function ReportCard({ month, year, state }: ReportCardProps) {
  return (
    <Panel as="section" size="md">
      <h2 className="text-text-primary mb-1 text-sm font-semibold">{t.reports.monthlyTitle}</h2>
      <p className="text-text-tertiary mb-4 text-xs">
        {MONTH_NAMES[month - 1]} {year}
      </p>

      <ReportCardBody month={month} year={year} state={state} />
    </Panel>
  );
}

function ReportCardBody({ month, year, state }: ReportCardProps) {
  switch (state.status) {
    case 'loading':
      return <ReportCardSkeleton />;
    case 'error':
      return (
        <EmptyState
          icon={FileSpreadsheet}
          title={t.dashboard.reportLoadError}
          description={t.dashboard.reportLoadErrorDescription}
        />
      );
    case 'empty':
      return (
        <EmptyState
          icon={FileSpreadsheet}
          title={t.dashboard.reportNoData}
          description={t.dashboard.reportNoDataDescription}
        />
      );
    case 'ready': {
      const { report } = state;
      const methodEntries = Object.entries(report.by_method);
      return (
        <>
          <ReportHeadline totals={report.totals} previous={report.previous_totals} />
          <ReportMobileSummary methodEntries={methodEntries} totals={report.totals} />
          <ReportDesktopTable month={month} year={year} methodEntries={methodEntries} totals={report.totals} />
          <ReportByCourt courts={report.by_court} />
        </>
      );
    }
  }
}
