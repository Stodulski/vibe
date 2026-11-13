import { Skeleton } from '@/shared/components/ui/skeleton';
import { cn } from '@/shared/lib/utils';
import {
  EXPORT_BUTTON_CLASS,
  MONTH_FIELD_CLASS,
  MONTH_TRIGGER_CLASS,
  REPORTS_BAR_CLASS,
  YEAR_FIELD_CLASS,
  YEAR_TRIGGER_CLASS,
} from './reportsLayout';

/** One picker of `MonthYearExportBar`: a label line over a 40px trigger. */
function PickerSkeleton({ fieldClass, triggerClass }: { fieldClass: string; triggerClass: string }) {
  return (
    <div className={fieldClass}>
      <Skeleton className="h-3.5 w-12 rounded-lg" />
      <Skeleton className={cn('h-10 rounded-md', triggerClass)} />
    </div>
  );
}

/** Loading stand-in for `MonthYearExportBar`: month and year pickers, then the export button. */
export function MonthYearExportBarSkeleton() {
  return (
    <div className={REPORTS_BAR_CLASS}>
      <PickerSkeleton fieldClass={MONTH_FIELD_CLASS} triggerClass={MONTH_TRIGGER_CLASS} />
      <PickerSkeleton fieldClass={YEAR_FIELD_CLASS} triggerClass={YEAR_TRIGGER_CLASS} />
      <Skeleton className={cn('h-10 rounded-full sm:w-44', EXPORT_BUTTON_CLASS)} />
    </div>
  );
}
