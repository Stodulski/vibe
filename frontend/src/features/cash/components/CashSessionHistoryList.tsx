import { useCallback, useMemo, type RefObject } from 'react';
import { AlertTriangle, Loader2 } from 'lucide-react';
import { Panel } from '@/shared/components/common/Panel';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { Button } from '@/shared/components/ui/button';
import { useIntersectionObserver } from '@/shared/hooks/useIntersectionObserver';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useCashSessions } from '../hooks/useCashSessions';
import { CashSessionHistoryRow } from './CashSessionHistoryRow';

const t = ES_AR;

/**
 * The next-page footer: a spinner while fetching, an explicit retry after a
 * failure (never auto-retried by the sentinel — see the caller), or nothing.
 * A next-page failure keeps every already-loaded page on screen and puts the
 * retry here, at the end of the list, instead of replacing the whole list
 * with the full-screen error (T3 review).
 */
function NextPageFooter({
  sentinelRef,
  isFetchingNextPage,
  isFetchNextPageError,
  onRetry,
}: {
  sentinelRef: RefObject<HTMLDivElement | null>;
  isFetchingNextPage: boolean;
  isFetchNextPageError: boolean;
  onRetry: () => void;
}) {
  return (
    <div ref={sentinelRef} className="flex justify-center py-3">
      {isFetchingNextPage ? (
        <Loader2 className="text-text-tertiary size-5 animate-spin" />
      ) : isFetchNextPageError ? (
        <Button variant="ghost" size="sm" onClick={onRetry}>
          {t.layout.retry}
        </Button>
      ) : null}
    </div>
  );
}

export function CashSessionHistoryList({ complexId }: { complexId: string }) {
  const { data, isLoading, isError, isFetchNextPageError, refetch, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useCashSessions(complexId);
  const sentinelRef = useIntersectionObserver(
    useCallback(() => {
      void fetchNextPage();
    }, [fetchNextPage]),
    // Never auto-retry off the sentinel while the last page fetch failed —
    // scrolling it back into view would otherwise refire the same failing
    // request in a loop instead of waiting for an explicit retry click.
    hasNextPage && !isFetchingNextPage && !isFetchNextPageError,
  );
  const sessions = useMemo(() => data?.pages.flatMap((p) => p.cash_sessions) ?? [], [data]);

  return (
    // `data-testid`: this list accumulates across e2e reruns/retries on the
    // shared complex, so e2e scopes to it (and to its first — newest — row)
    // instead of a page-wide text match that could hit a stale prior row
    // (see cash.spec.ts).
    <div data-testid="cash-history-list">
      <h2 className="text-text-tertiary mb-2 text-sm font-semibold tracking-wider uppercase">{t.cash.historyTitle}</h2>
      {isLoading ? (
        <SkeletonTable rows={3} />
      ) : isError && !data ? (
        <EmptyState
          icon={AlertTriangle}
          title={t.common.loadError}
          description={t.common.loadErrorDescription}
          actionLabel={t.layout.retry}
          onAction={() => {
            void refetch();
          }}
        />
      ) : sessions.length === 0 ? (
        <Panel size="sm">
          <p className="text-text-tertiary py-6 text-center text-sm">{t.cash.noHistory}</p>
        </Panel>
      ) : (
        <Panel size="sm">
          {sessions.map((session) => (
            <CashSessionHistoryRow key={session.id} session={session} />
          ))}
          {hasNextPage && (
            <NextPageFooter
              sentinelRef={sentinelRef}
              isFetchingNextPage={isFetchingNextPage}
              isFetchNextPageError={isFetchNextPageError}
              onRetry={() => {
                void fetchNextPage();
              }}
            />
          )}
        </Panel>
      )}
    </div>
  );
}
