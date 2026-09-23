import { ProductFormDialog } from './ProductFormDialog';
import { RestockDialog } from './RestockDialog';
import { AdjustDialog } from './AdjustDialog';
import { DeactivateProductDialog } from './DeactivateProductDialog';
import type { useProductDetailDialogs } from '../pages/products/useProductDetailDialogs';
import type { Product } from '@/shared/types/api.types';

interface ProductDetailDialogsProps {
  product: Product;
  complexId: string;
  dialogs: ReturnType<typeof useProductDetailDialogs>;
}

/** The four dialogs `ProductDetailPage`'s actions open, bundled so the page itself only wires state to them. */
export function ProductDetailDialogs({ product, complexId, dialogs }: ProductDetailDialogsProps) {
  return (
    <>
      <ProductFormDialog
        open={dialogs.editOpen}
        onClose={() => {
          dialogs.setEditOpen(false);
        }}
        complexId={complexId}
        product={product}
        existingCategories={product.category ? [product.category] : []}
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
