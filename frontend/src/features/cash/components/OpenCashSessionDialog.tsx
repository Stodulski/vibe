import { zodResolver } from '@hookform/resolvers/zod';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { useAppForm, submitHandler } from '@/shared/lib/form';
import { ES_AR } from '@/shared/i18n/es_AR';
import { MoneyPesosField } from './MoneyPesosField';
import { NoteField } from './NoteField';
import { openCashSessionSchema, type OpenCashSessionDto } from '../schemas/cash.schema';
import { pesosToCentavos } from '../lib/money';
import { blankNoteToUndefined } from '../lib/blankNoteToUndefined';
import { getFieldErrors } from '@/shared/lib/serverErrors';
import { useOpenCashSession } from '../hooks/useOpenCashSession';

const t = ES_AR;

interface OpenCashSessionDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
}

export function OpenCashSessionDialog({ open, onClose, complexId }: OpenCashSessionDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {/* Remounted per open: the body only exists in the tree while `open`
            is true, so each open starts a fresh form instance instead of an
            effect resetting a stale one. */}
        {open && <OpenCashSessionDialogBody onClose={onClose} complexId={complexId} />}
      </DialogContent>
    </Dialog>
  );
}

function OpenCashSessionDialogBody({ onClose, complexId }: { onClose: () => void; complexId: string }) {
  const openSession = useOpenCashSession(complexId);
  const form = useAppForm<OpenCashSessionDto>({
    resolver: zodResolver(openCashSessionSchema),
    // `Number.NaN` (not `undefined`), so an untouched field carries the same
    // type `z.number()` expects and the same "empty numeric input" value
    // `MoneyPesosField`'s `onChange` produces — matching `courts.schema.ts`'s
    // `bandPrice` pattern. Rejected by `sessionCashSchema`'s
    // `{ message: amountRequired }` exactly like a blank field would be.
    defaultValues: { opening_cash: Number.NaN, note: '' },
  });
  const {
    handleSubmit,
    setValue,
    watch,
    setError,
    formState: { errors },
  } = form;
  const openingCash = watch('opening_cash');

  const onSubmit = (data: OpenCashSessionDto) => {
    openSession.mutate(
      { opening_cash: pesosToCentavos(data.opening_cash), note: blankNoteToUndefined(data.note) },
      {
        onSuccess: onClose,
        onError: (error: unknown) => {
          const fieldErrors = getFieldErrors(error);
          if (fieldErrors.opening_cash) setError('opening_cash', { type: 'server', message: fieldErrors.opening_cash });
          if (fieldErrors.note) setError('note', { type: 'server', message: fieldErrors.note });
        },
      },
    );
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.cash.openAction}</DialogTitle>
      </DialogHeader>

      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4" noValidate>
        <MoneyPesosField
          id="open-cash-opening"
          label={t.cash.openingCash}
          value={openingCash}
          onChange={(pesos) => {
            setValue('opening_cash', pesos ?? Number.NaN, { shouldValidate: true });
          }}
          error={errors.opening_cash?.message}
          placeholder="0"
        />

        <NoteField
          id="open-cash-note"
          label={t.cash.openingNote}
          register={form.register('note')}
          error={errors.note?.message}
        />

        <SectionFooter
          onCancel={onClose}
          submitLabel={t.cash.openAction}
          pending={openSession.isPending}
          cancelDisabled={openSession.isPending}
        />
      </form>
    </>
  );
}
