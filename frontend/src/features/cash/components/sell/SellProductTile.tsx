import { memo } from 'react';
import { TappableCard } from '@/shared/components/common/TappableCard';
import { Tag } from '@/shared/components/common/Tag';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

interface SellProductTileProps {
  product: Product;
  onTap: (product: Product) => void;
}

/**
 * A single tappable catalog tile — big touch target, tap adds one unit
 * (`odd/tasks/pos-cashbox.md` T5b). Never disabled: selling past zero stock
 * is allowed, only flagged (`Tag` badge here, and again as the cart line's
 * own warning once it's added — `lineHasStockWarning`).
 */
export const SellProductTile = memo(function SellProductTile({ product, onTap }: SellProductTileProps) {
  const outOfStock = product.tracks_stock && product.stock_on_hand <= 0;

  return (
    <TappableCard
      onTap={() => {
        onTap(product);
      }}
      buttonLabel={product.name}
      buttonClassName="focus-self flex w-full flex-col items-start gap-2 rounded-2xl border border-border-subtle bg-bg-subtle p-4 text-left transition-colors hover:border-border-default hover:bg-bg-elevated focus-visible:border-primary-400 focus-visible:bg-bg-elevated active:scale-[0.98] min-h-24"
    >
      <p className="text-text-primary line-clamp-2 text-sm font-semibold">{product.name}</p>
      <div className="mt-auto flex w-full items-end justify-between gap-2">
        <span className="score-text text-text-primary text-base font-bold">{formatPrice(product.price)}</span>
        {outOfStock ? (
          <Tag tone="error">{t.cash.sellOutOfStock}</Tag>
        ) : (
          product.low_stock && <Tag tone="info">{`${t.cash.sellStockLeftPrefix} ${String(product.stock_on_hand)}`}</Tag>
        )}
      </div>
    </TappableCard>
  );
});
