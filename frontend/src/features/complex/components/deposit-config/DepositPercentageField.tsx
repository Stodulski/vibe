import { Percent } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UseFormRegister, FieldError } from 'react-hook-form';
import type { CreateComplexDto } from '../../schemas/complex.schema';

const t = ES_AR;

/**
 * The share of the price taken up front, 0 to 100.
 *
 * The input is sized for three digits, not for the column it sits in. It used
 * to stretch to 472px on a desktop — a field that wide tells the reader a long
 * answer is expected, and the longest answer here is "100".
 *
 * On FormField rather than a hand-rolled label so the error carries
 * `role="alert"`, which the previous version did not.
 */
export function DepositPercentageField({
  register,
  error,
}: {
  register: UseFormRegister<CreateComplexDto>;
  error: FieldError | undefined;
}) {
  return (
    <FormField
      label={t.complex.depositPercentage}
      htmlFor="deposit_percentage"
      helpText={t.complex.depositPercentageHelp}
      error={error?.message}
    >
      <div className="relative w-28">
        <Percent className="text-text-tertiary pointer-events-none absolute top-1/2 left-3 size-3.5 -translate-y-1/2" />
        <Input
          id="deposit_percentage"
          type="number"
          inputMode="numeric"
          min={0}
          max={100}
          className="pl-9 tabular-nums"
          aria-invalid={!!error}
          aria-describedby={error ? 'deposit_percentage-error' : undefined}
          {...register('deposit_percentage', { valueAsNumber: true })}
        />
      </div>
    </FormField>
  );
}
