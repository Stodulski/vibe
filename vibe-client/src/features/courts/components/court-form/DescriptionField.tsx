import type { UseFormRegister, FieldErrors } from 'react-hook-form';
import { Textarea } from '@/shared/components/ui/textarea';
import { Label } from '@/shared/components/ui/label';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CreateCourtDto } from '../../schemas/courts.schemas';

const t = ES_AR;

interface DescriptionFieldProps {
  register: UseFormRegister<CreateCourtDto>;
  errors: FieldErrors<CreateCourtDto>;
}

/**
 * What the court is like, in the owner's own words.
 *
 * A textarea rather than an input: the floor, the walls and the lighting are
 * three facts, and a single-line box that scrolls sideways invites one of them.
 * Two rows, not more — the field is capped at 200 characters and a box sized
 * for a paragraph would be promising room the cap does not give.
 *
 * Labelled optional, which the other fields are not. Every court needs a name
 * and a sport to exist; none needs a description, and a field that looks
 * required when it is not is how an owner ends up typing "-" to get past it.
 */
export function DescriptionField({ register, errors }: DescriptionFieldProps) {
  return (
    <div className="space-y-2">
      <Label htmlFor="court-description">{t.courts.descriptionOptional}</Label>
      <Textarea
        id="court-description"
        rows={2}
        // A hard stop rather than an error after the fact, the way every other
        // free-text field in this app caps itself. Two lines is the budget, and
        // finding that out on submit is worse than not being able to overrun it.
        maxLength={200}
        placeholder={t.placeholders.courtDescription}
        aria-invalid={!!errors.description}
        {...register('description')}
      />
      {errors.description && <p className="text-sm text-error-text">{errors.description.message}</p>}
    </div>
  );
}
