import type { UseFormRegister, FieldErrors } from 'react-hook-form';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CreateCourtDto } from '../../schemas/courts.schema';

const t = ES_AR;

interface NameFieldProps {
  register: UseFormRegister<CreateCourtDto>;
  errors: FieldErrors<CreateCourtDto>;
}

export function NameField({ register, errors }: NameFieldProps) {
  return (
    <div className="space-y-2">
      <Label htmlFor="court-name">{t.courts.name}</Label>
      <Input
        id="court-name"
        placeholder={t.placeholders.courtName}
        aria-invalid={!!errors.name}
        {...register('name')}
      />
      {errors.name && <p className="text-error-text text-sm">{errors.name.message}</p>}
    </div>
  );
}
