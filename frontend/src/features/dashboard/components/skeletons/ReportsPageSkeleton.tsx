import { Panel } from '@/shared/components/common/Panel';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ReportCardSkeleton } from '@/features/dashboard/pages/reports/ReportCardSkeleton';

/** One picker of `MonthYearExportBar`: a label line over a 40px trigger. */
function pickerPlaceholder(className: string) {
  return (
    <div className={`space-y-1 ${className}`}>
      <Skeleton className="h-3.5 w-12 rounded-lg" />
      <Skeleton className="h-10 w-full rounded-md" />
    </div>
  );
}

/**
 * What `ReportsPage` draws while the monthly report loads, chrome included:
 * the mobile title, the month/year/export bar and the report panel.
 */
export function ReportsPageSkeleton() {
  return (
    <div>
      <Skeleton className="mb-3 h-7 w-32 rounded-lg sm:mb-5 md:hidden" />
      <div className="mb-4 flex flex-wrap items-end gap-3 sm:mb-6">
        {pickerPlaceholder('flex-[2] sm:w-36 sm:flex-none')}
        {pickerPlaceholder('flex-1 sm:w-28 sm:flex-none')}
        <Skeleton className="h-10 w-full rounded-full sm:w-44" />
      </div>
      <Panel as="section" size="md">
        <div className="mb-1 flex h-5 items-center">
          <Skeleton className="h-4 w-36 rounded-lg" />
        </div>
        <div className="mb-4 flex h-4 items-center">
          <Skeleton className="h-3 w-24 rounded-lg" />
        </div>
        <ReportCardSkeleton />
      </Panel>
    </div>
  );
}
