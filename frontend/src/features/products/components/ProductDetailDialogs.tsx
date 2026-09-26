import { useMemo } from 'react';
import { ProductFormDialog } from './ProductFormDialog';
import { RestockDialog } from './RestockDialog';
import { AdjustDialog } from './AdjustDialog';
import { DeactivateProductDialog } from './DeactivateProductDialog';
import { useProducts } from '../hooks/useProducts';
import { buildCategorySet } from '../lib/categorySet';
import type { useProductDetailDialogs } from '../pages/products/useProductDetailDialogs';
import type { Product } from '@/shared/types/api.types';

interface ProductDetailDialogsProps {
  product: Product;
  complexId: string;
  dialogs: ReturnType<typeof useProductDetailDialogs>;
}

/** The four dialogs `ProductDetailPage`'s actions open, bundled so the page itself only wires state to them. */
export function ProductDetailDialogs({ product, complexId, dialogs }: ProductDetailDialogsProps) {
  // The whole active catalog, not just this product's own category
  // (`odd/tasks/product-category-chips.md` T2: editing from the detail page
  // used to only suggest the product's own category). Same query
  // `useProductsPage` already runs for the products list, so navigating here
  // from that list hits the cache instead of firing a new request.
  const catalogQuery = useProducts(complexId, true);
  const existingCategories = useMemo(() => buildCategorySet(catalogQuery.data?.products ?? []), [catalogQuery.data]);

  return (
    <>
      <ProductFormDialog
        open={dialogs.editOpen}
        onClose={() => {
          dialogs.setEditOpen(false);
        }}
        complexId={complexId}
        product={product}
        existingCategories={existingCategories}
      />

      {dialogs.restockOpen && (
        <RestockDialog
          open={dialogs.restockOpen}
          onClose={() => {
            dialogs.setRestockOpen(false);
          }}
          complexId={complexId}
          product={product}
        />
      )}

      {dialogs.adjustOpen && (
        <AdjustDialog
          open={dialogs.adjustOpen}
          onClose={() => {
            dialogs.setAdjustOpen(false);
          }}
          complexId={complexId}
          product={product}
        />
      )}

      <DeactivateProductDialog
        product={dialogs.toggleOpen ? product : null}
        onClose={() => {
          dialogs.setToggleOpen(false);
        }}
        complexId={complexId}
      />
    </>
  );
}
