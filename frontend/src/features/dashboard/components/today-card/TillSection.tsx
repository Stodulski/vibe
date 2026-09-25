import { Link } from 'react-router-dom';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import { formatVenueDayTime } from '@/shared/lib/formatVenueDayTime';
import type { CashSessionSummary } from '@/shared/types/api.types';

const t = ES_AR;

export function TillSectionSkeleton() {
  return (
    <div className="flex flex-col gap-2">
      <Skeleton className="h-4 w-24" />
      <Skeleton className="h-8 w-32" />
      <Skeleton className="h-8 w-28" />
    </div>
  );
}

/** The till is open: expected cash (the prominent figure), when it opened, and the way in. */
export function OpenTillSection({ summary, openedAt }: { summary: CashSessionSummary; openedAt: string }) {
  return (
    <div className="flex flex-col gap-2">
      <div>
        <p className="text-text-tertiary text-xs">{t.cash.expectedCash}</p>
        <p className="score-text text-text-primary text-2xl font-bold">{formatPrice(summary.expected_cash)}</p>
      </div>
      <p className="text-text-tertiary text-xs">
        {t.cash.openedAt}: {formatVenueDayTime(openedAt)}
      </p>
      <Button asChild size="sm" variant="outline" className="w-fit">
        <Link to="/cash">{t.dashboard.cashboxGoToShift}</Link>
      </Button>
    </div>
  );
}

/** The till is closed: nothing to reconcile against yet, just the way in. */
export function ClosedTillSection() {
  return (
    <div className="flex flex-col gap-2">
      <Button asChild size="sm" className="w-fit">
        <Link to="/cash">{t.cash.openAction}</Link>
      </Button>
    </div>
  );
}
