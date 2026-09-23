import { Link } from 'react-router-dom';
import type { UseFormReturn } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { useAppForm, submitHandler } from '@/shared/lib/form';
import { useCashSession } from '@/shared/hooks/useCashSession';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getFieldErrors } from '@/shared/lib/serverErrors';
import { MoneyPesosField } from '@/shared/components/common/MoneyPesosField';
import { NoteField } from '@/shared/components/common/NoteField';
import { pesosToCentavos } from '@/shared/lib/money';
import { blankToUndefined } from '@/shared/lib/blankToUndefined';
import { QuantityField } from './QuantityField';
import { RestockMethodField } from './RestockMethodField';
import { restockSchema, type RestockDto } from '../schemas/products.schema';
import { useRestockProduct } from '../hooks/useRestockProduct';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

interface RestockDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  product: Product;
}

export function RestockDialog({ open, onClose, complexId, product }: RestockDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {open && <RestockDialogBody key={product.id} onClose={onClose} complexId={complexId} product={product} />}
      </DialogContent>
    </Dialog>
  );
}

function useRestockForm(complexId: string, productId: string, onDone: () => void) {
  const restockProduct = useRestockProduct(complexId, productId);
  const form = useAppForm<RestockDto>({
    resolver: zodResolver(restockSchema),
    defaultValues: { quantity: Number.NaN, total_cost: Number.NaN, method: 'cash', note: '' },
  });
  const { setError } = form;

  const onSubmit = (data: RestockDto) => {
    restockProduct.mutate(
      {
        quantity: data.quantity,
        total_cost: pesosToCentavos(data.total_cost),
        method: data.method,
        note: blankToUndefined(data.note),
      },
      {
        onSuccess: onDone,
        onError: (error: unknown) => {
          const fieldErrors = getFieldErrors(error);
          if (fieldErrors.quantity) setError('quantity', { type: 'server', message: fieldErrors.quantity });
          if (fieldErrors.total_cost) setError('total_cost', { type: 'server', message: fieldErrors.total_cost });
          if (fieldErrors.method) setError('method', { type: 'server', message: fieldErrors.method });
          if (fieldErrors.note) setError('note', { type: 'server', message: fieldErrors.note });
        },
      },
    );
  };

  return { form, onSubmit, isPending: restockProduct.isPending };
}

function ClosedTillNotice() {
  return (
    <p className="border-warning-border bg-warning-bg text-warning-text rounded-xl border px-3.5 py-2.5 text-sm">
      {t.products.restockNeedsOpenTill}{' '}
      <Link to="/cash" className="font-medium underline">
        {t.products.goToCash}
      </Link>
    </p>
  );
}

/** Quantity, total cost, method and note — the restock's own fields, bound to `form`. */
function RestockFields({ form }: { form: UseFormReturn<RestockDto> }) {
  const { setValue, watch, register, formState } = form;
  const errors = formState.errors;
  const quantity = watch('quantity');
  const totalCost = watch('total_cost');
  const method = watch('method');

  return (
    <>
      <QuantityField
        id="restock-quantity"
        label={t.products.restockQuantityField}
        value={quantity}
        min={1}
        onChange={(v) => {
          setValue('quantity', v ?? Number.NaN, { shouldValidate: true });
        }}
        error={errors.quantity?.message}
        placeholder="0"
      />

      <MoneyPesosField
        id="restock-total-cost"
        label={t.products.restockTotalCostField}
        value={totalCost}
        onChange={(pesos) => {
          setValue('total_cost', pesos ?? Number.NaN, { shouldValidate: true });
        }}
        error={errors.total_cost?.message}
        placeholder="0"
      />

      <RestockMethodField
        value={method}
        onChange={(m) => {
          setValue('method', m, { shouldValidate: true });
        }}
        error={errors.method?.message}
      />

      <NoteField
        id="restock-note"
        label={t.products.restockNoteField}
        register={register('note')}
        error={errors.note?.message}
      />
    </>
  );
}

function RestockDialogBody({ onClose, complexId, product }: Omit<RestockDialogProps, 'open'>) {
  const cashSession = useCashSession(complexId);
  // Only a confirmed-closed till (a real 404) blocks submit — a background
  // error or an in-flight check must not block a legitimate restock (same
  // "don't conflate a real error with the actual state" shape as
  // `useCashSession`'s own `isRealError`/`isClosed` split).
  const tillClosed = cashSession.isClosed;
  const { form, onSubmit, isPending } = useRestockForm(complexId, product.id, onClose);

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.products.restockTitle}</DialogTitle>
      </DialogHeader>

      {tillClosed && <ClosedTillNotice />}

      <form onSubmit={submitHandler(form.handleSubmit, onSubmit)} className="space-y-4" noValidate>
        <RestockFields form={form} />

        <SectionFooter
          onCancel={onClose}
          submitLabel={t.products.restockAction}
          pending={isPending}
          submitDisabled={tillClosed}
          cancelDisabled={isPending}
        />
      </form>
    </>
  );
}
