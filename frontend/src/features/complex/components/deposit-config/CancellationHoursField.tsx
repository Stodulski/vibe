import { Clock } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UseFormRegister, FieldError } from 'react-hook-form';
import type { CreateComplexDto } from '../../schemas/complex.schema';

const t = ES_AR;

export function CancellationHoursField({
  register,
  error,
}: {
  register: UseFormRegister<CreateComplexDto>;
  error: FieldError | undefined;
}) {
  return (
    <FormField
      label={t.complex.cancellationHours}
      htmlFor="cancellation_hours"
      helpText={t.complex.cancellationHelp}
      error={error?.message}
    >
      {/* Sized for three digits — the ceiling is 168 hours. */}
      <div className="relative w-28">
        <Clock className="pointer-events-none absolute left-3 top-1/2 size-3.5 -translate-y-1/2 text-text-tertiary" />
        <Input
          id="cancellation_hours"
          type="number"
          inputMode="numeric"
          min={1}
          max={168}
          className="pl-9 tabular-nums"
          aria-invalid={!!error}
          aria-describedby={error ? 'cancellation_hours-error' : undefined}
          {...register('cancellation_hours', { valueAsNumber: true })}
        />
      </div>
    </FormField>
  );
}
