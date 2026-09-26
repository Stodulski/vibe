import { useCallback, useMemo, useState } from 'react';
import { AlertTriangle, Loader2 } from 'lucide-react';
import { Panel } from '@/shared/components/common/Panel';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { Button } from '@/shared/components/ui/button';
import { useIntersectionObserver } from '@/shared/hooks/useIntersectionObserver';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useSales } from '../../hooks/useSales';
import { SaleRow } from './SaleRow';
import { VoidSaleDialog } from './VoidSaleDialog';
import type { Sale } from '@/shared/types/api.types';

const t = ES_AR;

/** Same "keep every loaded page, retry only the failed one" shape as `CashSessionHistoryList`'s own `NextPageFooter`. */
function NextPageFooter({
  sentinelRef,
  isFetchingNextPage,
  isFetchNextPageError,
  onRetry,
}: {
  sentinelRef: React.RefObject<HTMLDivElement | null>;
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

interface SalesListBodyProps {
  isLoading: boolean;
  isError: boolean;
  hasData: boolean;
  sales: Sale[];
  onRetry: () => void;
  onVoid: (sale: Sale) => void;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  isFetchNextPageError: boolean;
  sentinelRef: React.RefObject<HTMLDivElement | null>;
  onRetryNextPage: () => void;
}

function SalesListBody({
  isLoading,
  isError,
  hasData,
  sales,
  onRetry,
  onVoid,
  hasNextPage,
  isFetchingNextPage,
  isFetchNextPageError,
  sentinelRef,
  onRetryNextPage,
}: SalesListBodyProps) {
  if (isLoading) return <SkeletonTable rows={3} mobile="flat" />;
  if (isError && !hasData) {
    return (
      <EmptyState
        icon={AlertTriangle}
        title={t.common.loadError}
        description={t.common.loadErrorDescription}
        actionLabel={t.layout.retry}
        onAction={onRetry}
      />
    );
  }
  if (sales.length === 0) {
    return (
      <Panel size="sm">
        <p className="text-text-tertiary py-6 text-center text-sm">{t.cash.noSales}</p>
      </Panel>
    );
  }
  return (
    <Panel size="sm">
      {sales.map((sale) => (
        <SaleRow
          key={sale.id}
          sale={sale}
          onVoid={() => {
            onVoid(sale);
          }}
        />
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
  );
}

/** This shift's own sales, newest first — below the grid/cart (`odd/tasks/pos-cashbox.md` T5b). */
export function SalesSection({ complexId, sessionId }: { complexId: string; sessionId: string }) {
  const { data, isLoading, isError, isFetchNextPageError, refetch, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useSales(complexId, sessionId);
  const sentinelRef = useIntersectionObserver(
    useCallback(() => {
      void fetchNextPage();
    }, [fetchNextPage]),
    hasNextPage && !isFetchingNextPage && !isFetchNextPageError,
  );
  const sales = useMemo(() => data?.pages.flatMap((p) => p.sales) ?? [], [data]);
  const [voidTarget, setVoidTarget] = useState<Sale | null>(null);

  return (
    <div data-testid="sell-sales-list">
      <h2 className="text-text-tertiary mb-2 text-sm font-semibold tracking-wider uppercase">{t.cash.salesTitle}</h2>
      <SalesListBody
        isLoading={isLoading}
        isError={isError}
        hasData={!!data}
        sales={sales}
        onRetry={() => {
          void refetch();
        }}
        onVoid={setVoidTarget}
        hasNextPage={hasNextPage}
        isFetchingNextPage={isFetchingNextPage}
        isFetchNextPageError={isFetchNextPageError}
        sentinelRef={sentinelRef}
        onRetryNextPage={() => {
          void fetchNextPage();
        }}
      />

      <VoidSaleDialog
        sale={voidTarget}
        onClose={() => {
          setVoidTarget(null);
        }}
        complexId={complexId}
        sessionId={sessionId}
      />
    </div>
  );
}
