import { PageHeader } from '@/shared/components/common/PageHeader';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { useSelectedComplex } from '@/features/complex';
import { useMonthlyReport } from '@/features/dashboard';
import { ES_AR } from '@/shared/i18n/es_AR';
import { MonthYearExportBar } from './reports/MonthYearExportBar';
import { ReportCard } from './reports/ReportCard';
import { useMonthYearSelection } from './reports/useMonthYearSelection';
import { useReportExport } from './reports/useReportExport';

const t = ES_AR;

export default function ReportsPage() {
  usePageTitle(t.navigation.reports);
  const { selectedComplexId, complex } = useSelectedComplex();

  // Guarded here so `useMonthlyReport`/`useReportExport` below only ever
  // mount once there's a real complex id, instead of `useReportExport`
  // carrying a `?? ''` fallback for a "no complex yet" state that can't
  // actually occur once it's mounted.
  if (!selectedComplexId) return null;

  return <ReportsPageContent complexId={selectedComplexId} createdAt={complex?.created_at} />;
}

function ReportsPageContent({ complexId, createdAt }: { complexId: string; createdAt: string | undefined }) {
  const { month, year, minYear, maxYear, availableMonths, handleYearChange, handleMonthChange } =
    useMonthYearSelection(createdAt);

  const { data: report, isLoading, isError } = useMonthlyReport(complexId, month, year);
  const { exporting, exportError, handleExport } = useReportExport(complexId, month, year);

  return (
    <div className="animate-fade-in">
      <PageHeader title={t.navigation.reports} />

      <MonthYearExportBar
        month={month}
        year={year}
        minYear={minYear}
        maxYear={maxYear}
        availableMonths={availableMonths}
        isLoading={isLoading}
        exporting={exporting}
        onMonthChange={handleMonthChange}
        onYearChange={handleYearChange}
        onExport={() => {
          void handleExport();
        }}
      />

      {exportError && (
        <div className="border-error-border bg-error-bg text-error-text mb-4 rounded-lg border px-4 py-3 text-sm">
          {exportError}
        </div>
      )}

      <ReportCard month={month} year={year} isLoading={isLoading} isError={isError} report={report} />
    </div>
  );
}
