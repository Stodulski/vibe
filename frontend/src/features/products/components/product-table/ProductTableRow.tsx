import { TableCell, TableRow } from '@/shared/components/ui/table';
import { cn, formatPrice } from '@/shared/lib/utils';
import { ProductBadges } from '../ProductBadges';
import { ProductActionsMenuFor } from '../ProductActionsMenuFor';
import { StockOnHand } from '../StockOnHand';
import type { Product } from '@/shared/types/api.types';

interface ProductTableRowProps {
  product: Product;
  onSelect: (product: Product) => void;
  onEdit: (product: Product) => void;
  onRestock: (product: Product) => void;
  onAdjust: (product: Product) => void;
  onToggleActive: (product: Product) => void;
}

/** The row's own actions cell — a sibling `stopPropagation` cell so opening the menu never also opens the detail (same shape as `ClientTableRow`). */
function ActionsCell({ product, onEdit, onRestock, onAdjust, onToggleActive }: Omit<ProductTableRowProps, 'onSelect'>) {
  return (
    <TableCell
      className="text-right"
      onClick={(e) => {
        e.stopPropagation();
      }}
    >
      <ProductActionsMenuFor
        product={product}
        onEdit={onEdit}
        onRestock={onRestock}
        onAdjust={onAdjust}
        onToggleActive={onToggleActive}
      />
    </TableCell>
  );
}

/** One product, as a dense table row — the desktop reading of `ProductCard`. Same shape as `ClientTableRow`. */
export function ProductTableRow({
  product,
  onSelect,
  onEdit,
  onRestock,
  onAdjust,
  onToggleActive,
}: ProductTableRowProps) {
  return (
    <TableRow
      onClick={() => {
        onSelect(product);
      }}
      className={cn('border-border-subtle hover:bg-bg-elevated cursor-pointer', !product.active && 'opacity-60')}
    >
      <TableCell className="max-w-56">
        <button
          type="button"
          title={product.name}
          onClick={(e) => {
            e.stopPropagation();
            onSelect(product);
          }}
          className="text-text-primary focus-visible:ring-primary-400 block max-w-full truncate rounded text-left text-sm font-semibold hover:underline focus-visible:ring-2 focus-visible:outline-none"
        >
          {product.name}
        </button>
      </TableCell>

      <TableCell className="text-text-secondary max-w-40 truncate text-sm">{product.category ?? '—'}</TableCell>

      <TableCell className="score-text text-text-primary text-right text-sm font-semibold">
        {formatPrice(product.price)}
      </TableCell>

      <TableCell className="score-text text-text-primary text-right text-sm font-semibold">
        <StockOnHand tracksStock={product.tracks_stock} stockOnHand={product.stock_on_hand} />
      </TableCell>

      <TableCell>
        <ProductBadges product={product} />
      </TableCell>

      <ActionsCell
        product={product}
        onEdit={onEdit}
        onRestock={onRestock}
        onAdjust={onAdjust}
        onToggleActive={onToggleActive}
      />
    </TableRow>
  );
}
