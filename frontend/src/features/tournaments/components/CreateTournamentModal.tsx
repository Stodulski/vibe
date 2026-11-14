import { Plus } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { CategoryFields } from './create-tournament-modal/CategoryFields';
import { TournamentBasicsFields } from './create-tournament-modal/TournamentBasicsFields';
import { useCreateTournamentForm } from './create-tournament-modal/useCreateTournamentForm';

const t = ES_AR.tournaments;

interface CreateTournamentModalProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
}

/** Mounted only while open, so every opening starts from a blank form. */
export function CreateTournamentModal({ open, onClose, complexId }: CreateTournamentModalProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-2xl">
        {open && <CreateTournamentBody complexId={complexId} onClose={onClose} />}
      </DialogContent>
    </Dialog>
  );
}

function CreateTournamentBody({ complexId, onClose }: { complexId: string; onClose: () => void }) {
  const { form, categories, onSubmit, isPending, error, addCategory } = useCreateTournamentForm(complexId, onClose);
  const listError = form.formState.errors.categories?.root?.message ?? form.formState.errors.categories?.message;

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.form.title}</DialogTitle>
        <DialogDescription>{t.form.description}</DialogDescription>
      </DialogHeader>
      <form onSubmit={submitHandler(form.handleSubmit, onSubmit)} className="space-y-4" noValidate>
        <TournamentBasicsFields form={form} />
        <h3 className="text-text-primary pt-1 text-sm font-semibold">{t.form.categories}</h3>
        {categories.fields.map((field, index) => (
          <CategoryFields
            key={field.id}
            form={form}
            index={index}
            canRemove={categories.fields.length > 1}
            onRemove={() => {
              categories.remove(index);
            }}
          />
        ))}
        {listError && (
          <p role="alert" className="text-error-text text-xs">
            {listError}
          </p>
        )}
        <Button type="button" variant="outline" className="w-full rounded-xl" onClick={addCategory}>
          <Plus className="size-4" aria-hidden="true" />
          {t.form.addCategory}
        </Button>
        {!!error && (
          <p role="alert" className="text-error-text text-sm">
            {getHttpErrorMessage(error, t.form.createError)}
          </p>
        )}
        <SectionFooter onCancel={onClose} cancelDisabled={isPending} submitLabel={t.form.submit} pending={isPending} />
      </form>
    </>
  );
}
