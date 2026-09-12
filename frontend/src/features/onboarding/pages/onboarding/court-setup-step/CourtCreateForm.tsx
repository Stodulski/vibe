import { Loader2, Plus } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CourtAttributeFields } from './CourtAttributeFields';
import type { UseFormRegister, UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { CreateCourtDto } from '@/features/courts';

const t = ES_AR;

export function CourtCreateForm({
  hasCourts,
  onSubmit,
  register,
  setValue,
  errors,
  isPending,
}: {
  hasCourts: boolean;
  onSubmit: React.SubmitEventHandler<HTMLFormElement>;
  register: UseFormRegister<CreateCourtDto>;
  setValue: UseFormSetValue<CreateCourtDto>;
  errors: FieldErrors<CreateCourtDto>;
  isPending: boolean;
}) {
  return (
    <form onSubmit={onSubmit} className="space-y-4 rounded-xl border border-border-subtle bg-bg-base p-4 sm:p-5">
      <p className="text-xs font-medium text-text-secondary">
        {hasCourts ? t.complex.addAnotherCourt : t.complex.addFirstCourt}
      </p>

      <div className="space-y-1.5">
        <Label htmlFor="court-name" className="text-xs">
          {t.courts.name}
        </Label>
        <Input
          id="court-name"
          placeholder={t.placeholders.courtName}
          aria-invalid={!!errors.name}
          {...register('name')}
        />
        {errors.name && <p className="text-xs text-error-text">{errors.name.message}</p>}
      </div>

      <CourtAttributeFields setValue={setValue} />

      <div className="flex justify-end">
        <Button type="submit" variant="outline" size="sm" disabled={isPending}>
          {isPending ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />}
          {t.courts.create}
        </Button>
      </div>
    </form>
  );
}
