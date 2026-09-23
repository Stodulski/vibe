import type { UseFormReturn } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { FormField } from '@/shared/components/common/FormField';
import { Input } from '@/shared/components/ui/input';
import { Switch } from '@/shared/components/ui/switch';
import { Label } from '@/shared/components/ui/label';
import { useAppForm, submitHandler } from '@/shared/lib/form';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getFieldErrors } from '@/shared/lib/serverErrors';
import { MoneyPesosField } from '@/shared/components/common/MoneyPesosField';
import { pesosToCentavos, centavosToPesos } from '@/shared/lib/money';
import { blankToUndefined } from '@/shared/lib/blankToUndefined';
import { QuantityField } from './QuantityField';
import { ProductCategoryField } from './ProductCategoryField';
import { productFormSchema, type ProductFormDto } from '../schemas/products.schema';
import { useCreateProduct } from '../hooks/useCreateProduct';
import { useUpdateProduct } from '../hooks/useUpdateProduct';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

interface ProductFormDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  /** `null`/`undefined` opens the dialog in "create" mode. */
  product?: Product | null | undefined;
  /** Categories already used across the catalog, for `ProductCategoryField`'s suggestions. */
  existingCategories: readonly string[];
}

export function ProductFormDialog({ open, onClose, complexId, product, existingCategories }: ProductFormDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {open && (
          <ProductFormDialogBody
            key={product?.id ?? 'create'}
            onClose={onClose}
            complexId={complexId}
            product={product}
            existingCategories={existingCategories}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function useProductForm(complexId: string, product: Product | null | undefined, onDone: () => void) {
  // A product that already carries a threshold can only replace it, never
  // clear it back to unset (`productsUpdate`'s own doc comment) — the schema
  // enforces that once `thresholdLocked`.
  const thresholdLocked = product?.low_stock_threshold != null;
  const createProduct = useCreateProduct(complexId);
  const updateProduct = useUpdateProduct(complexId);
  const isEdit = !!product;

  const form = useAppForm<ProductFormDto>({
    resolver: zodResolver(productFormSchema(thresholdLocked)),
    defaultValues: {
      name: product?.name ?? '',
      category: product?.category ?? '',
      price: product ? centavosToPesos(product.price) : Number.NaN,
      tracks_stock: product?.tracks_stock ?? true,
      low_stock_threshold: product?.low_stock_threshold ?? undefined,
    },
  });
  const { setError } = form;

  const onSubmit = (data: ProductFormDto) => {
    const onError = (error: unknown) => {
      const fieldErrors = getFieldErrors(error);
      if (fieldErrors.name) setError('name', { type: 'server', message: fieldErrors.name });
      if (fieldErrors.category) setError('category', { type: 'server', message: fieldErrors.category });
      if (fieldErrors.price) setError('price', { type: 'server', message: fieldErrors.price });
      if (fieldErrors.low_stock_threshold) {
        setError('low_stock_threshold', { type: 'server', message: fieldErrors.low_stock_threshold });
      }
    };

    if (isEdit) {
      updateProduct.mutate(
        {
          productId: product.id,
          data: {
            version: product.version,
            name: data.name.trim(),
            // Sent as-is (even empty) rather than `blankToUndefined`: PATCH
            // treats an OMITTED field as "keep current" and an EMPTY STRING
            // as "clear it" (`productsUpdate`'s own doc comment) — omitting
            // here would silently un-clear a category the person just erased.
            category: data.category?.trim() ?? '',
            price: pesosToCentavos(data.price),
            tracks_stock: data.tracks_stock,
            low_stock_threshold: data.low_stock_threshold,
          },
        },
        { onSuccess: onDone, onError },
      );
      return;
    }

    createProduct.mutate(
      {
        name: data.name.trim(),
        category: blankToUndefined(data.category?.trim()),
        price: pesosToCentavos(data.price),
        tracks_stock: data.tracks_stock,
        low_stock_threshold: data.low_stock_threshold,
      },
      { onSuccess: onDone, onError },
    );
  };

  return { form, onSubmit, isPending: isEdit ? updateProduct.isPending : createProduct.isPending, thresholdLocked };
}

function TracksStockToggle({ checked, onChange }: { checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <Label htmlFor="product-tracks-stock" className="text-xs">
        {t.products.tracksStockField}
      </Label>
      <Switch id="product-tracks-stock" checked={checked} onCheckedChange={onChange} />
    </div>
  );
}

function LowStockThresholdField({
  value,
  onChange,
  error,
  locked,
}: {
  value: number | undefined;
  onChange: (value: number | undefined) => void;
  error: string | undefined;
  locked: boolean;
}) {
  return (
    <QuantityField
      id="product-low-stock-threshold"
      label={t.products.lowStockThresholdField}
      value={value}
      onChange={onChange}
      error={error}
      placeholder="0"
      helpText={locked ? t.products.lowStockThresholdLockedHelp : undefined}
    />
  );
}

/** Name, category and price — the three fields every product (create or edit) always has. */
function ProductIdentityFields({
  form,
  existingCategories,
}: {
  form: UseFormReturn<ProductFormDto>;
  existingCategories: readonly string[];
}) {
  const { setValue, watch, register, formState } = form;
  const errors = formState.errors;
  const category = watch('category') ?? '';
  const price = watch('price');

  return (
    <>
      <FormField label={t.products.nameField} htmlFor="product-name" error={errors.name?.message}>
        <Input id="product-name" maxLength={120} {...register('name')} />
      </FormField>

      <ProductCategoryField
        value={category}
        onChange={(v) => {
          setValue('category', v, { shouldValidate: true });
        }}
        error={errors.category?.message}
        suggestions={existingCategories}
      />

      <MoneyPesosField
        id="product-price"
        label={t.products.priceField}
        value={price}
        onChange={(pesos) => {
          setValue('price', pesos ?? Number.NaN, { shouldValidate: true });
        }}
        error={errors.price?.message}
        placeholder="0"
      />
    </>
  );
}

function ProductFormDialogBody({
  onClose,
  complexId,
  product,
  existingCategories,
}: Omit<ProductFormDialogProps, 'open'>) {
  const { form, onSubmit, isPending, thresholdLocked } = useProductForm(complexId, product, onClose);
  const {
    handleSubmit,
    setValue,
    watch,
    formState: { errors },
  } = form;
  const tracksStock = watch('tracks_stock');
  const threshold = watch('low_stock_threshold');
  const isEdit = !!product;

  return (
    <>
      <DialogHeader>
        <DialogTitle>{isEdit ? t.products.editTitle : t.products.createTitle}</DialogTitle>
      </DialogHeader>

      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4" noValidate>
        <ProductIdentityFields form={form} existingCategories={existingCategories} />

        <TracksStockToggle
          checked={tracksStock}
          onChange={(checked) => {
            setValue('tracks_stock', checked, { shouldValidate: true });
          }}
        />

        {tracksStock && (
          <LowStockThresholdField
            value={threshold}
            onChange={(v) => {
              setValue('low_stock_threshold', v, { shouldValidate: true });
            }}
            error={errors.low_stock_threshold?.message}
            locked={thresholdLocked}
          />
        )}

        <SectionFooter onCancel={onClose} submitLabel={t.common.save} pending={isPending} cancelDisabled={isPending} />
      </form>
    </>
  );
}
