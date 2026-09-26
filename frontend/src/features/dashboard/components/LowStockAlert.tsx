import { Link } from 'react-router-dom';
import { AlertTriangle } from 'lucide-react';
import { Panel } from '@/shared/components/common/Panel';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Button } from '@/shared/components/ui/button';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useProducts } from '@/shared/hooks/useProducts';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

const MAX_LOW_STOCK_ITEMS = 5;

interface LowStockAlertProps {
  complexId: string;
}

function LowStockAlertSkeleton() {
  return (
    <Panel
      as="section"
      size="sm"
      className="flex h-full flex-col gap-2 p-4"
      aria-label={t.dashboard.lowStockTitle}
      role="status"
      aria-busy="true"
    >
      <Skeleton className="mb-1 h-4 w-24" />
      {[0, 1, 2].map((i) => (
        <Skeleton key={i} className="h-4 w-full" />
      ))}
    </Panel>
  );
}

/** Negative stock first, then lowest stock — the alert's own ordering before the 5-item cap. */
function sortLowStock(products: Product[]): Product[] {
  return [...products].sort((a, b) => {
    const aNegative = a.stock_on_hand < 0;
    const bNegative = b.stock_on_hand < 0;
    if (aNegative !== bNegative) return aNegative ? -1 : 1;
    return a.stock_on_hand - b.stock_on_hand;
  });
}

/**
 * Up to 5 tracked, active products flagged `low_stock` or
 * `needs_stock_review` (pos-cashbox T6), negative stock first. Hidden
 * entirely when there is nothing to flag — the same precedent the monthly
 * export's own empty blocks follow, rather than an empty card nobody needs.
 *
 * Self-contained like `RevenueChart`: owns its own `useProducts` query
 * instead of threading a product list through `DashboardContent`.
 */
export function LowStockAlert({ complexId }: LowStockAlertProps) {
  const query = useProducts(complexId, true);
  const retry = () => {
    void query.refetch();
  };

  if (query.isLoading) return <LowStockAlertSkeleton />;

  if (query.isError && !query.data) {
    return (
      <Panel
        as="section"
        size="sm"
        className="flex h-full flex-col items-center justify-center gap-2 p-4 text-center"
        aria-label={t.dashboard.lowStockTitle}
      >
        <AlertTriangle className="text-error-icon size-5" aria-hidden="true" />
        <p className="text-text-tertiary text-sm">{t.common.loadError}</p>
        <Button variant="outline" size="sm" onClick={retry}>
          {t.layout.retry}
        </Button>
      </Panel>
    );
  }

  const products = query.data?.products ?? [];
  const lowStock = sortLowStock(products.filter((p) => p.tracks_stock && (p.low_stock || p.needs_stock_review))).slice(
    0,
    MAX_LOW_STOCK_ITEMS,
  );

  if (lowStock.length === 0) return null;

  return (
    <Panel as="section" size="sm" className="flex h-full flex-col py-4 sm:p-4" aria-label={t.dashboard.lowStockTitle}>
      <h3 className="text-text-primary mb-2 text-sm font-semibold">{t.dashboard.lowStockTitle}</h3>

      {query.isError && <StaleDataNotice onRetry={retry} />}

      {/* eslint-disable-next-line jsx-a11y/no-redundant-roles -- same Tailwind-preflight/Safari note as TodayBookings' own list */}
      <ul className="flex-1 space-y-1" role="list">
        {lowStock.map((p) => (
          <li key={p.id} className="flex items-center justify-between gap-2 text-sm">
            <span className="text-text-primary truncate">{p.name}</span>
            <span className={p.stock_on_hand < 0 ? 'text-error-text font-medium' : 'text-text-tertiary'}>
              {p.stock_on_hand}
            </span>
          </li>
        ))}
      </ul>

      <Button asChild size="sm" variant="outline" className="mt-2 w-fit">
        <Link to="/cash/products">{t.dashboard.lowStockViewProducts}</Link>
      </Button>
    </Panel>
  );
}
