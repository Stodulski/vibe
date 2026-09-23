import { useMemo, useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useProducts } from '../../hooks/useProducts';
import type { Product } from '@/shared/types/api.types';

export type ProductStatusFilter = 'active' | 'inactive';

function resolveFilter(param: string | null): ProductStatusFilter {
  return param === 'inactive' ? 'inactive' : 'active';
}

/** All of `/cash/products`'s state: the Activos/Inactivos filter (in the URL, deep-linkable), client-side search, and every dialog's target. */
export function useProductsPage() {
  const navigate = useNavigate();
  const { selectedComplexId } = useSelectedComplex();
  const [searchParams, setSearchParams] = useSearchParams();
  const filter = resolveFilter(searchParams.get('status'));
  const [search, setSearch] = useState('');

  const setFilter = (next: ProductStatusFilter) => {
    setSearchParams(next === 'active' ? {} : { status: next }, { replace: true });
  };

  const query = useProducts(selectedComplexId, filter === 'active');
  const allProducts = useMemo(() => query.data?.products ?? [], [query.data]);

  const products = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return allProducts;
    return allProducts.filter(
      (p) => p.name.toLowerCase().includes(term) || (p.category ?? '').toLowerCase().includes(term),
    );
  }, [allProducts, search]);

  // Suggestions come from whichever list is currently loaded — combining
  // Activos and Inactivos would need fetching both filters at once for a
  // soft-suggestion field that already covers the common case (creating a
  // product for a category an active one already uses).
  const existingCategories = useMemo(() => {
    const set = new Set<string>();
    for (const p of allProducts) if (p.category) set.add(p.category);
    return [...set].sort((a, b) => a.localeCompare(b));
  }, [allProducts]);

  const [createOpen, setCreateOpen] = useState(false);
  const [editingProduct, setEditingProduct] = useState<Product | null>(null);
  const [restockTarget, setRestockTarget] = useState<Product | null>(null);
  const [adjustTarget, setAdjustTarget] = useState<Product | null>(null);
  const [toggleTarget, setToggleTarget] = useState<Product | null>(null);

  return {
    complexId: selectedComplexId,
    filter,
    setFilter,
    search,
    setSearch,
    isLoading: query.isLoading,
    isError: query.isError,
    refetch: query.refetch,
    products,
    hasAnyProducts: allProducts.length > 0,
    existingCategories,
    createOpen,
    setCreateOpen,
    editingProduct,
    setEditingProduct,
    restockTarget,
    setRestockTarget,
    adjustTarget,
    setAdjustTarget,
    toggleTarget,
    setToggleTarget,
    goToDetail: (product: Product) => {
      void navigate(`/cash/products/${product.id}`);
    },
  };
}
