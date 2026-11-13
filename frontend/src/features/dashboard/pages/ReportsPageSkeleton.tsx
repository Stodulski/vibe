import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { MonthYearExportBarSkeleton } from './reports/MonthYearExportBarSkeleton';
import { ReportCardShell } from './reports/ReportCardShell';
import { ReportCardSkeleton } from './reports/ReportCardSkeleton';

/**
 * What `ReportsPage` draws while the monthly report loads, chrome included:
 * the mobile title, the month/year/export bar and the report panel.
 */
export function ReportsPageSkeleton() {
  return (
    <div>
      <PageHeaderSkeleton />
      <MonthYearExportBarSkeleton />
      <ReportCardShell>
        <div className="mb-1 flex h-5 items-center">
          <Skeleton className="h-4 w-36 rounded-lg" />
        </div>
        <div className="mb-4 flex h-4 items-center">
          <Skeleton className="h-3 w-24 rounded-lg" />
        </div>
        <ReportCardSkeleton />
      </ReportCardShell>
    </div>
  );
}
