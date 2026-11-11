import { memo, useState } from 'react';
import { TappableCard } from '@/shared/components/common/TappableCard';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * The tile's one line of state, as plain text rather than a badge: how many
 * are already in the cart, else whether stock is out or running low.
 */
function TileStatus({ quantity, outOfStock, product }: { quantity: number; outOfStock: boolean; product: Product }) {
  if (quantity > 0) {
    return (
      <span aria-hidden="true" className="text-primary-400 text-xs font-medium">
        {`${String(quantity)} ${t.cash.sellInCartSuffix}`}
      </span>
    );
  }
  if (outOfStock) return <span className="text-error-text text-xs">{t.cash.sellOutOfStock}</span>;
  if (product.low_stock) {
    return (
      <span className="text-text-secondary text-xs">{`${t.cash.sellStockLeftPrefix} ${String(product.stock_on_hand)}`}</span>
    );
  }
  return null;
}

interface SellProductTileProps {
  product: Product;
  /** Units of this product already in the cart; 0 when it is not in it. */
  quantity: number;
  onTap: (product: Product) => void;
}

/**
 * A single tappable catalog tile — big touch target, tap adds one unit
 * (`odd/tasks/pos-cashbox.md` T5b). Never disabled: selling past zero stock
 * is allowed, only flagged (plain text here, and again as the cart line's
 * own warning once it's added — `lineHasStockWarning`).
 *
 * A quick tap on a phone holds `:active` for only a frame or two and there
 * is no hover on touch, so each tap also replays a short ring flash (keyed
 * by a tap counter) that confirms the add where the finger is.
 */
export const SellProductTile = memo(function SellProductTile({ product, quantity, onTap }: SellProductTileProps) {
  const outOfStock = product.tracks_stock && product.stock_on_hand <= 0;
  const [tapCount, setTapCount] = useState(0);

  return (
    // Below `sm` a tile is one row of a list: name on the left, price and
    // state on the right. From `sm` it is a grid card at the full height of its
    // row, the price line pushed to the bottom so prices line up across a row.
    <TappableCard
      className="h-full"
      onTap={() => {
        setTapCount((count) => count + 1);
        onTap(product);
      }}
      // The name stays the product's alone: changing it on every tap would
      // rename the button under a screen reader and under anything that finds
      // it by name. The cart lines already announce each quantity.
      buttonLabel={product.name}
      // A product already in the cart wears a green edge and says how many,
      // so the grid shows what was added without a look at the cart.
      buttonClassName={`focus-self relative flex h-full w-full touch-manipulation items-center justify-between gap-3 rounded-2xl sm:flex-col sm:items-start sm:justify-start sm:gap-2 border ${quantity > 0 ? 'border-primary-500' : 'border-border-subtle'} bg-bg-subtle px-3.5 py-3.5 text-left sm:py-3 transition-[color,background-color,border-color,transform] duration-150 hover:border-border-default hover:bg-bg-highlight focus-visible:border-primary-400 focus-visible:bg-bg-elevated active:scale-[0.96] active:border-primary-400 active:bg-bg-elevated`}
    >
      {tapCount > 0 && (
        <span
          key={tapCount}
          aria-hidden="true"
          className="animate-tile-added ring-primary-400 pointer-events-none absolute inset-0 rounded-2xl ring-2"
        />
      )}
      <p className="text-text-primary line-clamp-2 min-w-0 flex-1 text-sm font-semibold sm:flex-none">{product.name}</p>
      <span className="flex shrink-0 flex-row-reverse items-center gap-2 sm:mt-auto sm:w-full sm:flex-row sm:items-center sm:justify-between sm:gap-2">
        <span className="score-text text-text-primary text-sm leading-5 font-bold whitespace-nowrap">
          {formatPrice(product.price)}
        </span>
        <span className="text-xs leading-4 sm:h-5 sm:leading-5 [&>span]:leading-4 sm:[&>span]:leading-5">
          <TileStatus quantity={quantity} outOfStock={outOfStock} product={product} />
        </span>
      </span>
    </TappableCard>
  );
});
