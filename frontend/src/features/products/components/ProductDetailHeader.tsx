import { Panel } from '@/shared/components/common/Panel';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ProductBadges } from './ProductBadges';
import { ProductActionsMenu } from './ProductActionsMenu';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

interface ProductDetailHeaderProps {
  product: Product;
  onEdit: () => void;
  onRestock: () => void;
  onAdjust: () => void;
  onToggleActive: () => void;
}

export function ProductDetailHeader({
  product,
  onEdit,
  onRestock,
  onAdjust,
  onToggleActive,
}: ProductDetailHeaderProps) {
  return (
    <Panel>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-text-primary truncate text-lg font-semibold">{product.name}</h1>
          {product.category && <p className="text-text-tertiary mt-0.5 text-sm">{product.category}</p>}
        </div>
        <ProductActionsMenu
          product={product}
          onEdit={onEdit}
          onRestock={onRestock}
          onAdjust={onAdjust}
          onToggleActive={onToggleActive}
        />
      </div>

      <div className="mt-4 grid grid-cols-2 gap-3">
        <div>
          <p className="text-micro text-text-tertiary font-medium tracking-wider uppercase">{t.products.priceLabel}</p>
          <p className="score-text text-text-primary mt-1 text-lg font-bold">{formatPrice(product.price)}</p>
        </div>
        <div>
          <p className="text-micro text-text-tertiary font-medium tracking-wider uppercase">{t.products.stockLabel}</p>
          <p className="score-text text-text-primary mt-1 text-lg font-bold">
            {product.tracks_stock ? product.stock_on_hand : t.products.noStockControl}
          </p>
        </div>
      </div>

      <div className="mt-3">
        <ProductBadges product={product} />
      </div>
    </Panel>
  );
}
