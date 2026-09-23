import { useMemo, useState } from 'react';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useCashSession } from '@/shared/hooks/useCashSession';
import { useProducts } from '@/shared/hooks/useProducts';
import { cartTotal } from '../../lib/cart';
import { useSellCart } from './useSellCart';
import { useSellCharge } from './useSellCharge';
import type { Product } from '@/shared/types/api.types';

function sortByCategoryThenName(products: Product[]): Product[] {
  return [...products].sort((a, b) => {
    const categoryCompare = (a.category ?? '').localeCompare(b.category ?? '');
    if (categoryCompare !== 0) return categoryCompare;
    return a.name.localeCompare(b.name);
  });
}

/** The active catalog: the query itself, plus client-side search/category filtering. */
function useCatalog(complexId: string | null) {
  const productsQuery = useProducts(complexId, true);
  const products = useMemo(() => productsQuery.data?.products ?? [], [productsQuery.data]);
  const sortedProducts = useMemo(() => sortByCategoryThenName(products), [products]);
  const categories = useMemo(() => {
    const set = new Set<string>();
    for (const p of products) if (p.category) set.add(p.category);
    return [...set].sort((a, b) => a.localeCompare(b));
  }, [products]);

  const [search, setSearch] = useState('');
  const [category, setCategory] = useState<string | null>(null);
  const visibleProducts = useMemo(() => {
    const term = search.trim().toLowerCase();
    return sortedProducts.filter((p) => {
      if (category !== null && p.category !== category) return false;
      if (term && !p.name.toLowerCase().includes(term)) return false;
      return true;
    });
  }, [sortedProducts, search, category]);

  return { productsQuery, products, categories, search, setSearch, category, setCategory, visibleProducts };
}

/**
 * All of `/cash/sell`'s state: catalog, the cart (`useSellCart`), the charge
 * flow (`useSellCharge`), and search/category filters. Split into three
 * hooks so none of them grows past this repo's `max-lines-per-function`
 * limit — see each one's own doc comment.
 */
export function useSellPage() {
  const { selectedComplexId } = useSelectedComplex();
  const complexId = selectedComplexId;
  const cashSession = useCashSession(complexId);
  const sessionId = cashSession.data?.cash_session.id ?? null;

  const catalog = useCatalog(complexId);
  const cart = useSellCart(complexId, catalog.products, catalog.productsQuery.isSuccess);
  const charge = useSellCharge(complexId, sessionId, cart);
  const total = useMemo(() => cartTotal(cart.lines, catalog.products), [cart.lines, catalog.products]);

  return {
    complexId,
    cashSession,
    productsQuery: catalog.productsQuery,
    hasAnyProducts: catalog.products.length > 0,
    visibleProducts: catalog.visibleProducts,
    categories: catalog.categories,
    search: catalog.search,
    setSearch: catalog.setSearch,
    category: catalog.category,
    setCategory: catalog.setCategory,
    addProduct: (product: Product) => {
      cart.add(product.id);
    },
    lines: cart.lines,
    incrementCartLine: cart.increment,
    decrementCartLine: cart.decrement,
    setCartLineQuantity: cart.setQuantity,
    removeCartLine: charge.removeCartLine,
    total,
    products: catalog.products,
    method: charge.method,
    setMethod: charge.setMethod,
    note: charge.note,
    setNote: charge.setNote,
    lineErrors: charge.lineErrors,
    droppedStaleNotice: cart.droppedStaleNotice,
    dismissDroppedStaleNotice: cart.dismissDroppedStaleNotice,
    cartLimitNotice: cart.cartLimitNotice,
    mobileCartOpen: charge.mobileCartOpen,
    setMobileCartOpen: charge.setMobileCartOpen,
    charge: charge.charge,
    isCharging: charge.isCharging,
    saleResult: charge.saleResult,
    clearSaleResult: charge.clearSaleResult,
  };
}
