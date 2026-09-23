import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useUpdateProduct } from '../hooks/useUpdateProduct';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

interface DeactivateProductDialogProps {
  product: Product | null;
  onClose: () => void;
  complexId: string;
}

/**
 * One dialog for both directions — `product.active` decides the copy, same
 * as `ConfirmDialog`'s own destructive/default split. A no-op `active` PATCH
 * still sends `version` for optimistic concurrency, same as any other edit.
 */
export function DeactivateProductDialog({ product, onClose, complexId }: DeactivateProductDialogProps) {
  const updateProduct = useUpdateProduct(complexId);
  const open = !!product;
  const willDeactivate = product?.active ?? true;

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={() => {
        if (!product) return;
        updateProduct.mutate(
          {
            productId: product.id,
            data: { version: product.version, active: !product.active },
            toggledActive: !product.active,
          },
          { onSuccess: onClose },
        );
      }}
      title={willDeactivate ? t.products.deactivateTitle : t.products.reactivateTitle}
      description={willDeactivate ? t.products.deactivateDescription : t.products.reactivateDescription}
      confirmLabel={willDeactivate ? t.products.deactivateAction : t.products.reactivateAction}
      variant={willDeactivate ? 'destructive' : 'default'}
      isLoading={updateProduct.isPending}
    />
  );
}
