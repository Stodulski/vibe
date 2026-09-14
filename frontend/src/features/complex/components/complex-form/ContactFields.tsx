import type { Control, FieldErrors, UseFormRegister } from 'react-hook-form';
import { Controller } from 'react-hook-form';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { PhoneInput } from '@/shared/components/common/PhoneInput';
import { RequiredMark } from '@/shared/components/common/RequiredMark';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CreateComplexDto } from '../../schemas/complex.schema';

const t = ES_AR;

interface ContactFieldsProps {
  register: UseFormRegister<CreateComplexDto>;
  control: Control<CreateComplexDto>;
  errors: FieldErrors<CreateComplexDto>;
}

export function ContactFields({ register, control, errors }: ContactFieldsProps) {
  return (
    <>
      <FormField
        label={
          <>
            {t.complex.phone} <RequiredMark />
          </>
        }
        htmlFor="phone"
        error={errors.phone?.message}
      >
        <Controller
          name="phone"
          control={control}
          render={({ field }) => (
            // Sized for a phone number, not for the column, from `sm` up: the
            // width of a field is a promise about how much is expected in it.
            // On a phone the column is already that narrow, and a box shorter
            // than its neighbours reads as broken rather than as a hint.
            <div className="sm:max-w-72">
              <PhoneInput
                id="phone"
                value={field.value}
                onChange={field.onChange}
                onBlur={field.onBlur}
                aria-invalid={!!errors.phone}
              />
            </div>
          )}
        />
      </FormField>

      <FormField
        label={
          <>
            {t.complex.email} <span className="text-text-tertiary">(opcional)</span>
          </>
        }
        htmlFor="email"
        error={errors.email?.message}
      >
        <Input id="email" type="email" aria-invalid={!!errors.email} {...register('email')} />
      </FormField>
    </>
  );
}
