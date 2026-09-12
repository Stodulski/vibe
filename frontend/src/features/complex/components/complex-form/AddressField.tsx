import type { Control, FieldErrors, UseFormSetValue, UseFormWatch } from 'react-hook-form';
import { Controller } from 'react-hook-form';
import { FormField } from '@/shared/components/common/FormField';
import { RequiredMark } from '@/shared/components/common/RequiredMark';
import { AddressInput } from '../AddressInput';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CreateComplexDto } from '../../schemas/complex.schema';

const t = ES_AR;

interface AddressFieldProps {
  control: Control<CreateComplexDto>;
  errors: FieldErrors<CreateComplexDto>;
  watch: UseFormWatch<CreateComplexDto>;
  setValue: UseFormSetValue<CreateComplexDto>;
}

export function AddressField({ control, errors, watch, setValue }: AddressFieldProps) {
  return (
    <FormField
      label={
        <>
          {t.complex.address} <RequiredMark />
        </>
      }
      htmlFor="formatted_address"
      error={errors.formatted_address?.message ?? errors.latitude?.message}
    >
      <Controller
        name="formatted_address"
        control={control}
        render={({ field }) => (
          <AddressInput
            id="formatted_address"
            value={field.value}
            confirmed={watch('latitude') != null}
            onChange={field.onChange}
            onSelect={(details) => {
              field.onChange(details.formatted_address);
              setValue('address', details.address);
              setValue('city', details.city);
              setValue('province', details.province);
              setValue('latitude', details.latitude);
              setValue('longitude', details.longitude);
            }}
            onClear={() => {
              setValue('address', '');
              setValue('city', '');
              setValue('province', '');
              setValue('latitude', undefined);
              setValue('longitude', undefined);
            }}
            aria-invalid={!!errors.formatted_address || !!errors.latitude}
          />
        )}
      />
    </FormField>
  );
}
