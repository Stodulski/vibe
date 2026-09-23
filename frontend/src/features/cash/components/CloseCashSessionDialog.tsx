import { useState } from 'react';
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
import { CashHintRow } from './CashHintRow';
import { CashDifference } from './CashDifference';
import { ClosedSessionResult } from './ClosedSessionResult';
import { closeCashSessionSchema, type CloseCashSessionDto } from '../schemas/cash.schema';
import { useCloseCashSession } from '../hooks/useCloseCashSession';
import type { CashSession } from '@/shared/types/api.types';

const t = ES_AR;

interface CloseCashSessionDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  sessionId: string;
  expectedCash: number;
}

export function CloseCashSessionDialog({
  open,
  onClose,
  complexId,
  sessionId,
  expectedCash,
}: CloseCashSessionDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {open && (
          <CloseCashSessionDialogBody
            onClose={onClose}
            complexId={complexId}
            sessionId={sessionId}
            expectedCash={expectedCash}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function useCloseForm(complexId: string, sessionId: string, onClosed: (session: CashSession) => void) {
  const closeSession = useCloseCashSession(complexId, sessionId);
  const form = useAppForm<CloseCashSessionDto>({
    resolver: zodResolver(closeCashSessionSchema),
    defaultValues: { counted_cash: Number.NaN, note: '' },
  });
  const { setError } = form;

  const onSubmit = (data: CloseCashSessionDto) => {
    closeSession.mutate(
      { counted_cash: pesosToCentavos(data.counted_cash), note: blankToUndefined(data.note) },
      {
        onSuccess: (result) => {
          onClosed(result.cash_session);
        },
        onError: (error: unknown) => {
          const fieldErrors = getFieldErrors(error);
          if (fieldErrors.counted_cash) setError('counted_cash', { type: 'server', message: fieldErrors.counted_cash });
          if (fieldErrors.note) setError('note', { type: 'server', message: fieldErrors.note });
        },
      },
    );
  };

  return { form, onSubmit, isPending: closeSession.isPending };
}

function CloseCashSessionDialogBody({
  onClose,
  complexId,
  sessionId,
  expectedCash,
}: Omit<CloseCashSessionDialogProps, 'open'>) {
  // Two steps in one dialog instance rather than closing immediately on
  // success: "after close, show the closed result and return to the closed
  // state" (odd/tasks/pos-cashbox.md T3) — the numbers the person just
  // committed to, not a toast they may have missed.
  const [closedSession, setClosedSession] = useState<CashSession | null>(null);
  const { form, onSubmit, isPending } = useCloseForm(complexId, sessionId, setClosedSession);
  const {
    handleSubmit,
    setValue,
    watch,
    formState: { errors },
  } = form;
  const countedCash = watch('counted_cash');
  const liveDifference = Number.isNaN(countedCash) ? undefined : pesosToCentavos(countedCash) - expectedCash;

  if (closedSession) return <ClosedSessionResult session={closedSession} onDone={onClose} />;

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.cash.closeTitle}</DialogTitle>
      </DialogHeader>

      <CashHintRow label={t.cash.closeExpectedHint} value={expectedCash} emphasize />

      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4" noValidate>
        <MoneyPesosField
          id="close-cash-counted"
          label={t.cash.countedCash}
          value={countedCash}
          onChange={(pesos) => {
            setValue('counted_cash', pesos ?? Number.NaN, { shouldValidate: true });
          }}
          error={errors.counted_cash?.message}
          placeholder="0"
        />

        {liveDifference !== undefined && <CashDifference difference={liveDifference} />}

        <NoteField
          id="close-cash-note"
          label={t.cash.closeNote}
          register={form.register('note')}
          error={errors.note?.message}
        />

        <p className="text-text-tertiary text-xs">{t.cash.closeConfirmWarning}</p>

        <SectionFooter
          onCancel={onClose}
          submitLabel={t.cash.closeAction}
          submitVariant="destructive"
          pending={isPending}
          cancelDisabled={isPending}
        />
      </form>
    </>
  );
}
