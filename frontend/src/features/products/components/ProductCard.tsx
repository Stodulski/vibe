import { memo } from 'react';
import { TappableCard } from '@/shared/components/common/TappableCard';
import { formatPrice, cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ProductBadges } from './ProductBadges';
import { ProductActionsMenuFor } from './ProductActionsMenuFor';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

interface ProductCardProps {
  product: Product;
  onSelect: (product: Product) => void;
  onEdit: (product: Product) => void;
  onRestock: (product: Product) => void;
  onAdjust: (product: Product) => void;
  onToggleActive: (product: Product) => void;
}

function Figure({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="text-micro text-text-tertiary font-medium tracking-wider uppercase">{label}</p>
      <div className="mt-1">{children}</div>
    </div>
  );
}

export const ProductCard = memo(function ProductCard({
  product,
  onSelect,
  onEdit,
  onRestock,
  onAdjust,
  onToggleActive,
}: ProductCardProps) {
  return (
    <TappableCard
      onTap={() => {
        onSelect(product);
      }}
      buttonLabel={product.name}
      className={cn(!product.active && 'opacity-60')}
      actionClassName="absolute right-3 top-3"
      action={
        <ProductActionsMenuFor
          product={product}
          onEdit={onEdit}
          onRestock={onRestock}
          onAdjust={onAdjust}
          onToggleActive={onToggleActive}
        />
      }
      buttonClassName={cn(
        'focus-self flex w-full flex-col items-stretch rounded-2xl border border-border-subtle',
        'bg-bg-subtle p-4 text-left transition-colors hover:border-border-default hover:bg-bg-elevated',
        'focus-visible:border-primary-400 focus-visible:bg-bg-elevated',
      )}
    >
      <div className="flex items-start gap-2 pr-10">
        <div className="min-w-0 flex-1">
          <p className="text-text-primary truncate text-sm font-semibold">{product.name}</p>
          {product.category && <p className="text-text-tertiary mt-0.5 truncate text-xs">{product.category}</p>}
        </div>
      </div>

      <div className="mt-3 flex items-end justify-between gap-2">
        <Figure label={t.products.priceLabel}>
          <span className="score-text text-text-primary text-lg font-bold">{formatPrice(product.price)}</span>
        </Figure>
        <div className="text-right">
          <Figure label={t.products.stockLabel}>
            <span className="score-text text-text-primary text-sm font-semibold">
              {product.tracks_stock ? product.stock_on_hand : t.products.noStockControl}
            </span>
          </Figure>
        </div>
      </div>

      <div className="mt-3">
        <ProductBadges product={product} />
      </div>
    </TappableCard>
  );
});
