import { zodResolver } from '@hookform/resolvers/zod';
import { createCourtSchema, type CreateCourtDto } from '../schemas/courts.schema';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useAppForm, submitHandler } from '@/shared/lib/form';
import { NameField } from './court-form/NameField';
import { SportAndTypeFields } from './court-form/SportAndTypeFields';
import { DescriptionField } from './court-form/DescriptionField';
import { useCourtFormSubmit, courtDefaultValues } from './court-form/useCourtFormSubmit';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR;

interface CourtFormProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  court?: CourtWithPrices | undefined;
  onCreated?: ((court: CourtWithPrices) => void) | undefined;
}

/**
 * Mounted only while `open`, keyed by which court it edits (or `'new'`).
 *
 * The form used to stay mounted across every open/close cycle, with an
 * effect calling `reset()` whenever `open`/`court` changed. Radix unmounts
 * `DialogContent`'s children when closed, but `useAppForm()` lived above that in
 * `CourtForm` itself, so its state survived the unmount — the reset effect
 * existed only to undo that survival, and it ran one render after the values
 * it was correcting were already on screen. Remounting a fresh instance per
 * court gets the right `defaultValues` from the start, with no reset frame.
 */
export function CourtForm({ open, onClose, complexId, court, onCreated }: CourtFormProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent>
        {open && (
          <CourtFormBody
            key={court?.id ?? 'new'}
            complexId={complexId}
            court={court}
            onClose={onClose}
            onCreated={onCreated}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function CourtFormBody({
  complexId,
  court,
  onClose,
  onCreated,
}: {
  complexId: string;
  court: CourtWithPrices | undefined;
  onClose: () => void;
  onCreated?: ((court: CourtWithPrices) => void) | undefined;
}) {
  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useAppForm<CreateCourtDto>({
    resolver: zodResolver(createCourtSchema),
    defaultValues: courtDefaultValues(court),
  });

  const { isEdit, mutation, onSubmit } = useCourtFormSubmit(complexId, court, onClose, onCreated);

  return (
    <>
      <DialogHeader>
        <DialogTitle>{isEdit ? t.courts.edit : t.courts.create}</DialogTitle>
      </DialogHeader>

      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4">
        <NameField register={register} errors={errors} />

        <SportAndTypeFields control={control} errors={errors} />

        <DescriptionField register={register} errors={errors} />

        <SectionFooter
          onCancel={onClose}
          submitLabel={isEdit ? t.common.save : t.common.create}
          pending={mutation.isPending}
        />
      </form>
    </>
  );
}
