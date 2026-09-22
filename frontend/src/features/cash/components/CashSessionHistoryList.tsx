import { useCallback, useMemo } from 'react';
import { AlertTriangle, Loader2 } from 'lucide-react';
import { Panel } from '@/shared/components/common/Panel';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { useIntersectionObserver } from '@/shared/hooks/useIntersectionObserver';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useCashSessions } from '../hooks/useCashSessions';
import { CashSessionHistoryRow } from './CashSessionHistoryRow';

const t = ES_AR;

export function CashSessionHistoryList({ complexId }: { complexId: string }) {
  const { data, isLoading, isError, refetch, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useCashSessions(complexId);
  const sentinelRef = useIntersectionObserver(
    useCallback(() => {
      void fetchNextPage();
    }, [fetchNextPage]),
    hasNextPage && !isFetchingNextPage,
  );
  const sessions = useMemo(() => data?.pages.flatMap((p) => p.cash_sessions) ?? [], [data]);

  return (
    <div>
      <h2 className="text-text-tertiary mb-2 text-sm font-semibold tracking-wider uppercase">{t.cash.historyTitle}</h2>
      {isLoading ? (
        <SkeletonTable rows={3} />
      ) : isError ? (
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
            <div ref={sentinelRef} className="flex justify-center py-3">
              {isFetchingNextPage && <Loader2 className="text-text-tertiary size-5 animate-spin" />}
            </div>
          )}
        </Panel>
      )}
    </div>
  );
}
