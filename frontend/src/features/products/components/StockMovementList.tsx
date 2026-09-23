import { useCallback, useMemo, type RefObject } from 'react';
import { AlertTriangle, Loader2 } from 'lucide-react';
import { Panel } from '@/shared/components/common/Panel';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { Button } from '@/shared/components/ui/button';
import { useIntersectionObserver } from '@/shared/hooks/useIntersectionObserver';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useProductStockMovements } from '../hooks/useProductStockMovements';
import { StockMovementRow } from './StockMovementRow';
import type { StockMovement } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * The next-page footer: a spinner while fetching, an explicit retry after a
 * failure (never auto-retried by the sentinel), or nothing — same shape as
 * `features/cash`'s `CashSessionHistoryList`'s own footer. A next-page
 * failure keeps every already-loaded page on screen instead of replacing the
 * whole list with the full-screen error (T5a: "keep loaded pages when the
 * next page fails").
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

interface LoadedMovementsProps {
  movements: StockMovement[];
  isError: boolean;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  isFetchNextPageError: boolean;
  sentinelRef: RefObject<HTMLDivElement | null>;
  onRetry: () => void;
  onRetryNextPage: () => void;
}

/**
 * The loaded-history view: the rows themselves, the next-page footer, and —
 * separately from a next-page failure — the shared stale-data notice for a
 * background refetch that failed while rows are still cached (same
 * non-blocking guard as `ProductDetailPage`'s own header, instead of wiping
 * the already-loaded list).
 */
function LoadedMovements({
  movements,
  isError,
  hasNextPage,
  isFetchingNextPage,
  isFetchNextPageError,
  sentinelRef,
  onRetry,
  onRetryNextPage,
}: LoadedMovementsProps) {
  return (
    <>
      {isError && <StaleDataNotice onRetry={onRetry} />}
      <Panel size="sm" className="p-0">
        {movements.map((movement) => (
          <StockMovementRow key={movement.id} movement={movement} />
        ))}
        {hasNextPage && (
          <NextPageFooter
            sentinelRef={sentinelRef}
            isFetchingNextPage={isFetchingNextPage}
            isFetchNextPageError={isFetchNextPageError}
            onRetry={onRetryNextPage}
          />
        )}
      </Panel>
    </>
  );
}

export function StockMovementList({ complexId, productId }: { complexId: string; productId: string }) {
  const { data, isLoading, isError, isFetchNextPageError, refetch, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useProductStockMovements(complexId, productId);
  const sentinelRef = useIntersectionObserver(
    useCallback(() => {
      void fetchNextPage();
    }, [fetchNextPage]),
    hasNextPage && !isFetchingNextPage && !isFetchNextPageError,
  );
  const movements = useMemo(() => data?.pages.flatMap((p) => p.stock_movements) ?? [], [data]);

  return (
    <div data-testid="stock-movement-list">
      <h2 className="text-text-tertiary mb-2 text-sm font-semibold tracking-wider uppercase">
        {t.products.historyTitle}
      </h2>
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
      ) : movements.length === 0 ? (
        <Panel size="sm">
          <p className="text-text-tertiary py-6 text-center text-sm">{t.products.noHistory}</p>
        </Panel>
      ) : (
        <LoadedMovements
          movements={movements}
          isError={isError}
          hasNextPage={hasNextPage}
          isFetchingNextPage={isFetchingNextPage}
          isFetchNextPageError={isFetchNextPageError}
          sentinelRef={sentinelRef}
          onRetry={() => {
            void refetch();
          }}
          onRetryNextPage={() => {
            void fetchNextPage();
          }}
        />
      )}
    </div>
  );
}
