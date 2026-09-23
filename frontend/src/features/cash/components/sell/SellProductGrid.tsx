import { Search, PackageSearch } from 'lucide-react';
import { IconInput } from '@/shared/components/common/IconInput';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { SellCategoryChips } from './SellCategoryChips';
import { SellProductTile } from './SellProductTile';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

interface SellProductGridProps {
  products: Product[];
  hasAnyProducts: boolean;
  search: string;
  onSearchChange: (value: string) => void;
  categories: string[];
  category: string | null;
  onCategoryChange: (category: string | null) => void;
  onSelect: (product: Product) => void;
}

export function SellProductGrid({
  products,
  hasAnyProducts,
  search,
  onSearchChange,
  categories,
  category,
  onCategoryChange,
  onSelect,
}: SellProductGridProps) {
  return (
    <div className="space-y-3">
      <IconInput
        icon={Search}
        value={search}
        onChange={(e) => {
          onSearchChange(e.target.value);
        }}
        placeholder={t.cash.sellSearchPlaceholder}
        aria-label={t.cash.sellSearchPlaceholder}
      />
      {categories.length > 0 && (
        <SellCategoryChips categories={categories} selected={category} onSelect={onCategoryChange} />
      )}
      {!hasAnyProducts ? (
        <EmptyState icon={PackageSearch} title={t.cash.sellEmptyCatalog} description={t.products.emptyDescription} />
      ) : products.length === 0 ? (
        <EmptyState icon={PackageSearch} title={t.cash.sellNoResults} description="" />
      ) : (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {products.map((product) => (
            <SellProductTile key={product.id} product={product} onTap={onSelect} />
          ))}
        </div>
      )}
    </div>
  );
}
