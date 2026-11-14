import { AlertCircle } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { EventFormFields } from './create-event-modal/EventFormFields';
import { useCreateEventForm } from './create-event-modal/useCreateEventForm';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR.events;

interface CreateEventModalProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  courts: CourtWithPrices[];
}

/** Mounted only while open, so every opening starts from a blank form. */
export function CreateEventModal({ open, onClose, complexId, courts }: CreateEventModalProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-lg">
        {open && <CreateEventBody complexId={complexId} courts={courts} onClose={onClose} />}
      </DialogContent>
    </Dialog>
  );
}

function ConflictNotice({ conflict, error }: { conflict: string | null; error: unknown }) {
  if (!error) return null;
  return (
    <div role="alert" className="border-error-border bg-error-bg flex items-start gap-2.5 rounded-xl border p-3">
      <AlertCircle className="text-error-icon mt-0.5 size-4 shrink-0" aria-hidden="true" />
      <div className="min-w-0 text-sm">
        {conflict ? (
          <>
            <p className="text-error-text font-medium">{t.conflict.title}</p>
            <p className="text-text-secondary mt-0.5 break-words">{conflict}</p>
            <p className="text-text-tertiary mt-1 text-xs">{t.conflict.hint}</p>
          </>
        ) : (
          <>
            <p className="text-error-text font-medium">{t.form.createError}</p>
            <p className="text-text-secondary mt-0.5 break-words">{getHttpErrorMessage(error, '')}</p>
          </>
        )}
      </div>
    </div>
  );
}

function CreateEventBody({
  complexId,
  courts,
  onClose,
}: {
  complexId: string;
  courts: CourtWithPrices[];
  onClose: () => void;
}) {
  const { form, onSubmit, isPending, error, conflict } = useCreateEventForm(complexId, onClose);

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.create}</DialogTitle>
        <DialogDescription>{t.createDescription}</DialogDescription>
      </DialogHeader>

      <form onSubmit={submitHandler(form.handleSubmit, onSubmit)} className="space-y-4" noValidate>
        <EventFormFields form={form} courts={courts} />
        <ConflictNotice conflict={conflict} error={error} />
        <SectionFooter onCancel={onClose} cancelDisabled={isPending} submitLabel={t.form.submit} pending={isPending} />
      </form>
    </>
  );
}
