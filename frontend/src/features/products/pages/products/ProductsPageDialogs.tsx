import { ProductFormDialog } from '../../components/ProductFormDialog';
import { RestockDialog } from '../../components/RestockDialog';
import { AdjustDialog } from '../../components/AdjustDialog';
import { DeactivateProductDialog } from '../../components/DeactivateProductDialog';
import type { useProductsPage } from './useProductsPage';

/** Every dialog `/cash/products`'s list actions open, bundled so the page itself only wires state to them. */
export function ProductsPageDialogs({
  state,
  complexId,
}: {
  state: ReturnType<typeof useProductsPage>;
  complexId: string;
}) {
  return (
    <>
      <ProductFormDialog
        open={state.createOpen}
        onClose={() => {
          state.setCreateOpen(false);
        }}
        complexId={complexId}
        existingCategories={state.existingCategories}
      />

      <ProductFormDialog
        open={!!state.editingProduct}
        onClose={() => {
          state.setEditingProduct(null);
        }}
        complexId={complexId}
        product={state.editingProduct}
        existingCategories={state.existingCategories}
      />

      {state.restockTarget && (
        <RestockDialog
          open={!!state.restockTarget}
          onClose={() => {
            state.setRestockTarget(null);
          }}
          complexId={complexId}
          product={state.restockTarget}
        />
      )}

      {state.adjustTarget && (
        <AdjustDialog
          open={!!state.adjustTarget}
          onClose={() => {
            state.setAdjustTarget(null);
          }}
          complexId={complexId}
          product={state.adjustTarget}
        />
      )}

      <DeactivateProductDialog
        product={state.toggleTarget}
        onClose={() => {
          state.setToggleTarget(null);
        }}
        complexId={complexId}
      />
    </>
  );
}
