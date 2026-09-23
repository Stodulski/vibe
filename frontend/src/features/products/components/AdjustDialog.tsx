import { zodResolver } from '@hookform/resolvers/zod';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { useAppForm, submitHandler } from '@/shared/lib/form';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getFieldErrors } from '@/shared/lib/serverErrors';
import { NoteField } from '@/shared/components/common/NoteField';
import { blankToUndefined } from '@/shared/lib/blankToUndefined';
import { QuantityField } from './QuantityField';
import { AdjustReasonField } from './AdjustReasonField';
import { adjustSchema, type AdjustDto } from '../schemas/products.schema';
import { ADJUSTMENT_REASONS } from '../lib/adjustmentReasons';
import { stockDifference } from '../lib/stockDifference';
import { useAdjustProduct } from '../hooks/useAdjustProduct';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

interface AdjustDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  product: Product;
}

export function AdjustDialog({ open, onClose, complexId, product }: AdjustDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {open && <AdjustDialogBody key={product.id} onClose={onClose} complexId={complexId} product={product} />}
      </DialogContent>
    </Dialog>
  );
}

function useAdjustForm(complexId: string, productId: string, currentStock: number, onDone: () => void) {
  const adjustProduct = useAdjustProduct(complexId, productId);
  const form = useAppForm<AdjustDto>({
    resolver: zodResolver(adjustSchema(currentStock)),
    // `ADJUSTMENT_REASONS[0]` ("Rotura"), not `undefined` — same reasoning as
    // `MovementFormDialog`'s `categories[0]` default: `reason` is a required
    // enum, so the select always shows a real, changeable choice rather than
    // an empty placeholder state a required field can't actually hold.
    defaultValues: { counted: Number.NaN, reason: ADJUSTMENT_REASONS[0], note: '' },
  });
  const { setError } = form;

  const onSubmit = (data: AdjustDto) => {
    const quantity = stockDifference(data.counted, currentStock);
    // The submit button is already disabled at 0/undefined (see
    // `AdjustDialogBody`) — this is a defensive backstop, never expected to
    // fire from a real click.
    if (!quantity) return;

    adjustProduct.mutate(
      { quantity, reason: data.reason, note: blankToUndefined(data.note) },
      {
        onSuccess: onDone,
        onError: (error: unknown) => {
          const fieldErrors = getFieldErrors(error);
          if (fieldErrors.quantity) setError('counted', { type: 'server', message: fieldErrors.quantity });
          if (fieldErrors.reason) setError('reason', { type: 'server', message: fieldErrors.reason });
          if (fieldErrors.note) setError('note', { type: 'server', message: fieldErrors.note });
        },
      },
    );
  };

  return { form, onSubmit, isPending: adjustProduct.isPending };
}

function AdjustDialogBody({ onClose, complexId, product }: Omit<AdjustDialogProps, 'open'>) {
  const { form, onSubmit, isPending } = useAdjustForm(complexId, product.id, product.stock_on_hand, onClose);
  const {
    handleSubmit,
    setValue,
    watch,
    formState: { errors },
  } = form;
  const counted = watch('counted');
  const reason = watch('reason');
  const difference = stockDifference(counted, product.stock_on_hand);

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.products.adjustTitle}</DialogTitle>
      </DialogHeader>

      <p className="text-text-secondary text-sm" data-testid="adjust-current-stock">
        {t.products.currentStock}: <span className="text-text-primary font-semibold">{product.stock_on_hand}</span>
      </p>

      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4" noValidate>
        <QuantityField
          id="adjust-counted"
          label={t.products.adjustCountedField}
          value={counted}
          onChange={(v) => {
            setValue('counted', v ?? Number.NaN, { shouldValidate: true });
          }}
          error={errors.counted?.message}
          placeholder="0"
        />

        {difference !== undefined && (
          <p className="text-sm" data-testid="adjust-difference">
            {t.products.adjustDifferenceLabel}: <DifferenceValue value={difference} />
          </p>
        )}

        <AdjustReasonField
          value={reason}
          onChange={(r) => {
            setValue('reason', r, { shouldValidate: true });
          }}
          error={errors.reason?.message}
        />

        <NoteField
          id="adjust-note"
          label={t.products.adjustNoteField}
          register={form.register('note')}
          error={errors.note?.message}
        />

        <SectionFooter
          onCancel={onClose}
          submitLabel={t.products.adjustAction}
          pending={isPending}
          submitDisabled={!difference}
          cancelDisabled={isPending}
        />
      </form>
    </>
  );
}

function DifferenceValue({ value }: { value: number }) {
  const sign = value > 0 ? '+' : '';
  return (
    <span className={value > 0 ? 'text-success-text font-semibold' : value < 0 ? 'text-error-text font-semibold' : ''}>
      {sign}
      {value}
    </span>
  );
}
