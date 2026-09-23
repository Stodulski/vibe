import { AlertTriangle, TrendingDown } from 'lucide-react';
import { Tag } from '@/shared/components/common/Tag';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * `low_stock`/`needs_stock_review` are computed server-side (`Product`'s own
 * doc comment in `openapi.yaml`) — this only reads the flags the API already
 * sends, never re-derives them from `stock_on_hand`/`low_stock_threshold`
 * itself, so the client can never disagree with the server about what counts
 * as low or negative stock.
 */
export function ProductBadges({ product }: { product: Product }) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {!product.active && <Tag tone="neutral">{t.products.inactiveBadge}</Tag>}
      {product.needs_stock_review ? (
        <Tag tone="error" className="gap-1">
          <AlertTriangle className="size-3" aria-hidden="true" />
          {t.products.needsReviewBadge}
        </Tag>
      ) : (
        product.low_stock && (
          <Tag tone="info" className="gap-1">
            <TrendingDown className="size-3" aria-hidden="true" />
            {t.products.lowStockBadge}
          </Tag>
        )
      )}
    </div>
  );
}
