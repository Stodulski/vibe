import { memo, useState } from 'react';
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
 *
 * A quick tap on a phone holds `:active` for only a frame or two and there
 * is no hover on touch, so each tap also replays a short ring flash (keyed
 * by a tap counter) that confirms the add where the finger is.
 */
export const SellProductTile = memo(function SellProductTile({ product, onTap }: SellProductTileProps) {
  const outOfStock = product.tracks_stock && product.stock_on_hand <= 0;
  const [tapCount, setTapCount] = useState(0);

  return (
    <TappableCard
      onTap={() => {
        setTapCount((count) => count + 1);
        onTap(product);
      }}
      buttonLabel={product.name}
      buttonClassName="focus-self relative flex w-full touch-manipulation flex-col items-start gap-2 rounded-2xl border border-border-subtle bg-bg-subtle p-4 text-left transition-[color,background-color,border-color,transform] duration-150 hover:border-border-default hover:bg-bg-elevated focus-visible:border-primary-400 focus-visible:bg-bg-elevated active:scale-[0.96] active:border-primary-400 active:bg-bg-elevated min-h-24"
    >
      {tapCount > 0 && (
        <span
          key={tapCount}
          aria-hidden="true"
          className="animate-tile-added ring-primary-400 pointer-events-none absolute inset-0 rounded-2xl ring-2"
        />
      )}
      <p className="text-text-primary line-clamp-2 text-sm font-semibold">{product.name}</p>
      <div className="mt-auto flex w-full flex-wrap items-end justify-between gap-x-2 gap-y-1">
        <span className="score-text text-text-primary text-base font-bold whitespace-nowrap">
          {formatPrice(product.price)}
        </span>
        {outOfStock ? (
          <Tag tone="error">{t.cash.sellOutOfStock}</Tag>
        ) : (
          product.low_stock && <Tag tone="info">{`${t.cash.sellStockLeftPrefix} ${String(product.stock_on_hand)}`}</Tag>
        )}
      </div>
    </TappableCard>
  );
});
