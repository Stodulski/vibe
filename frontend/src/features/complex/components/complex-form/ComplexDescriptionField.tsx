import { useWatch, type Control, type FieldErrors, type UseFormRegister } from 'react-hook-form';
import { Textarea } from '@/shared/components/ui/textarea';
import { Label } from '@/shared/components/ui/label';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CreateComplexDto } from '../../schemas/complex.schema';
import { DESCRIPTION_MAX_LENGTH, MIN_DESCRIPTION_LENGTH } from '../../lib/description';

const t = ES_AR;

interface ComplexDescriptionFieldProps {
  register: UseFormRegister<CreateComplexDto>;
  control: Control<CreateComplexDto>;
  errors: FieldErrors<CreateComplexDto>;
}

/**
 * The complex's own description, shown on its public page.
 *
 * The counter says how far the text is from the profile checklist's minimum
 * while it is short, and only the count once it is long enough. The maximum is
 * a hard stop (`maxLength`), the same as every other free-text field here.
 */
export function ComplexDescriptionField({ register, control, errors }: ComplexDescriptionFieldProps) {
  const value = useWatch({ control, name: 'description' }) ?? '';
  const length = value.trim().length;

  return (
    <div className="space-y-2">
      <Label htmlFor="complex-description">{t.complex.description}</Label>
      <Textarea
        id="complex-description"
        rows={4}
        maxLength={DESCRIPTION_MAX_LENGTH}
        placeholder={t.complex.descriptionPlaceholder}
        aria-invalid={!!errors.description}
        aria-describedby="complex-description-hint"
        {...register('description')}
      />
      <p id="complex-description-hint" className="text-text-tertiary flex justify-between gap-3 text-sm">
        <span>{length < MIN_DESCRIPTION_LENGTH ? t.complex.descriptionMinHint(MIN_DESCRIPTION_LENGTH) : null}</span>
        <span className="shrink-0 tabular-nums">{t.complex.descriptionCount(length, DESCRIPTION_MAX_LENGTH)}</span>
      </p>
      {errors.description && <p className="text-error-text text-sm">{errors.description.message}</p>}
    </div>
  );
}
