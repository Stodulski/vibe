import { Search, PackageSearch } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { cn } from '@/shared/lib/utils';
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
  /** Units in the cart by product id, for the tiles' green "in cart" mark. */
  cartQuantities: ReadonlyMap<string, number>;
}

interface SellProductFiltersProps {
  search: string;
  onSearchChange: (value: string) => void;
  categories: string[];
  category: string | null;
  onCategoryChange: (category: string | null) => void;
}

/**
 * True once the element has scrolled up under the app header (64px): a zero
 * height sentinel placed right above the sticky bar stops intersecting then.
 */
function useStuck() {
  const sentinelRef = useRef<HTMLDivElement>(null);
  const [stuck, setStuck] = useState(false);
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el || typeof IntersectionObserver === 'undefined') return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        setStuck(entry ? !entry.isIntersecting : false);
      },
      { rootMargin: '-64px 0px 0px 0px' },
    );
    observer.observe(el);
    return () => {
      observer.disconnect();
    };
  }, []);
  return { sentinelRef, stuck };
}

/**
 * Search and categories, pinned under the app header while the grid scrolls,
 * so filtering never needs a trip back to the top. At rest it is invisible
 * chrome; only once pinned does it take a solid background and a bottom
 * rule, so the tiles passing under it end at a clean edge.
 */
function SellProductFilters({
  search,
  onSearchChange,
  categories,
  category,
  onCategoryChange,
}: SellProductFiltersProps) {
  const { sentinelRef, stuck } = useStuck();

  return (
    <>
      <div ref={sentinelRef} aria-hidden="true" />
      <div
        className={cn(
          'sticky top-16 z-10 space-y-3 border-b py-3 transition-colors',
          stuck ? 'border-border-subtle bg-bg-base' : 'border-transparent',
        )}
      >
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
      </div>
    </>
  );
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
  cartQuantities,
}: SellProductGridProps) {
  const navigate = useNavigate();

  // Nothing to search or filter yet: the only useful step is loading the
  // catalog, so the search box stays out and the empty state links there.
  if (!hasAnyProducts) {
    return (
      <section>
        <h2 className="sr-only">{t.cash.sellProductsTitle}</h2>
        <EmptyState
          icon={PackageSearch}
          title={t.cash.sellEmptyCatalog}
          description={t.products.emptyDescription}
          actionLabel={t.cash.sellAddProducts}
          onAction={() => {
            void navigate('/cash/products');
          }}
        />
      </section>
    );
  }

  return (
    // No gap of its own: the filter bar's padding spaces it from the grid, and
    // a constant height keeps the grid still when the bar pins.
    <section className="-mt-3">
      <h2 className="sr-only">{t.cash.sellProductsTitle}</h2>
      <SellProductFilters
        search={search}
        onSearchChange={onSearchChange}
        categories={categories}
        category={category}
        onCategoryChange={onCategoryChange}
      />
      {products.length === 0 ? (
        <EmptyState icon={PackageSearch} title={t.cash.sellNoResults} description="" />
      ) : (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-3 sm:gap-3 lg:grid-cols-4">
          {products.map((product) => (
            <SellProductTile
              key={product.id}
              product={product}
              quantity={cartQuantities.get(product.id) ?? 0}
              onTap={onSelect}
            />
          ))}
        </div>
      )}
    </section>
  );
}
