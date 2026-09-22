import { AlertTriangle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * Non-blocking indicator for a background refetch failure while cached data
 * is still on screen. `CashPageBody`/`OpenCashView`/`CashSessionDetailPage`
 * used to swap the whole view for the full-screen `EmptyState` error the
 * moment a query's `isError` flipped true, even though React Query keeps
 * serving the last good `data` through a failed background refetch — that
 * unmounted whatever the person had open (a dialog, typed input) for a
 * failure that was often transient (T3 review: "background refetch error
 * wipes the view"). Same "keep the last good render, don't blank the page"
 * shape as `DashboardLayout`'s `ComplexLoadError` guard (PR #119).
 */
export function StaleDataNotice({ onRetry }: { onRetry: () => void }) {
  return (
    <div
      role="status"
      className="border-warning-border bg-warning-bg text-warning-text flex items-center justify-between gap-3 rounded-xl border px-3.5 py-2.5 text-sm"
    >
      <span className="flex items-center gap-2">
        <AlertTriangle className="size-4 shrink-0" aria-hidden="true" />
        {t.common.staleDataNotice}
      </span>
      <Button variant="ghost" size="xs" onClick={onRetry}>
        {t.layout.retry}
      </Button>
    </div>
  );
}
