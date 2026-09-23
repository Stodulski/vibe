import { zodResolver } from '@hookform/resolvers/zod';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { useAppForm, submitHandler } from '@/shared/lib/form';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getFieldErrors } from '@/shared/lib/serverErrors';
import { MoneyPesosField } from '@/shared/components/common/MoneyPesosField';
import { NoteField } from '@/shared/components/common/NoteField';
import { pesosToCentavos } from '@/shared/lib/money';
import { blankToUndefined } from '@/shared/lib/blankToUndefined';
import { MovementCategoryField } from './MovementCategoryField';
import { MovementMethodField } from './MovementMethodField';
import { cashMovementSchema, type CashMovementDto } from '../schemas/cash.schema';
import { categoriesFor } from '../lib/movementCategories';
import { useCreateCashMovement } from '../hooks/useCreateCashMovement';

const t = ES_AR;

interface MovementFormDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  sessionId: string;
  kind: 'income' | 'expense';
}

/**
 * "Ingreso" and "Egreso" are two call sites of the same dialog (fixed `kind`
 * per instance — the task's own screens section names them as two separate
 * actions, not one toggle), so category, method and amount stay one form
 * instead of two near-identical copies.
 */
export function MovementFormDialog({ open, onClose, complexId, sessionId, kind }: MovementFormDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {open && (
          <MovementFormDialogBody
            key={kind}
            onClose={onClose}
            complexId={complexId}
            sessionId={sessionId}
            kind={kind}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function useMovementForm(complexId: string, sessionId: string, kind: 'income' | 'expense', onDone: () => void) {
  const categories = categoriesFor(kind);
  const createMovement = useCreateCashMovement(complexId, sessionId);
  const form = useAppForm<CashMovementDto>({
    resolver: zodResolver(cashMovementSchema),
    // `categories[0]` is `MovementCategory | undefined` under
    // `noUncheckedIndexedAccess`; `categoriesFor` never actually returns an
    // empty array (both `INCOME_CATEGORIES` and `EXPENSE_CATEGORIES` are
    // non-empty tuples), so the fallback is unreachable at runtime.
    defaultValues: { kind, category: categories[0] ?? 'other_income', method: 'cash', amount: Number.NaN, note: '' },
  });
  const { setError } = form;

  const onSubmit = (data: CashMovementDto) => {
    createMovement.mutate(
      {
        kind: data.kind,
        category: data.category,
        method: data.method,
        amount: pesosToCentavos(data.amount),
        note: blankToUndefined(data.note),
      },
      {
        onSuccess: onDone,
        onError: (error: unknown) => {
          const fieldErrors = getFieldErrors(error);
          if (fieldErrors.category) setError('category', { type: 'server', message: fieldErrors.category });
          if (fieldErrors.method) setError('method', { type: 'server', message: fieldErrors.method });
          if (fieldErrors.amount) setError('amount', { type: 'server', message: fieldErrors.amount });
          if (fieldErrors.note) setError('note', { type: 'server', message: fieldErrors.note });
        },
      },
    );
  };

  return { form, categories, onSubmit, isPending: createMovement.isPending };
}

function MovementFormDialogBody({ onClose, complexId, sessionId, kind }: Omit<MovementFormDialogProps, 'open'>) {
  const { form, categories, onSubmit, isPending } = useMovementForm(complexId, sessionId, kind, onClose);
  const {
    handleSubmit,
    setValue,
    watch,
    formState: { errors },
  } = form;
  const amount = watch('amount');
  const category = watch('category');
  const method = watch('method');
  const actionLabel = kind === 'income' ? t.cash.incomeAction : t.cash.expenseAction;

  return (
    <>
      <DialogHeader>
        <DialogTitle>{actionLabel}</DialogTitle>
      </DialogHeader>

      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4" noValidate>
        <MovementCategoryField
          categories={categories}
          value={category}
          onChange={(c) => {
            setValue('category', c, { shouldValidate: true });
          }}
          error={errors.category?.message}
        />

        <MovementMethodField
          value={method}
          onChange={(m) => {
            setValue('method', m, { shouldValidate: true });
          }}
          error={errors.method?.message}
        />

        <MoneyPesosField
          id="movement-amount"
          label={t.cash.movementAmount}
          value={amount}
          onChange={(pesos) => {
            setValue('amount', pesos ?? Number.NaN, { shouldValidate: true });
          }}
          error={errors.amount?.message}
          placeholder="0"
        />

        <NoteField
          id="movement-note"
          label={t.cash.movementNote}
          register={form.register('note')}
          error={errors.note?.message}
        />

        <SectionFooter onCancel={onClose} submitLabel={actionLabel} pending={isPending} cancelDisabled={isPending} />
      </form>
    </>
  );
}
