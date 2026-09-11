import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { PersonalInfoFormData } from './schema';

const t = ES_AR;

interface PersonalInfoNameFieldsProps {
  register: UseFormRegister<PersonalInfoFormData>;
  errors: FieldErrors<PersonalInfoFormData>;
}

export function PersonalInfoNameFields({ register, errors }: PersonalInfoNameFieldsProps) {
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <FormField label={t.auth.firstName} htmlFor="first_name" error={errors.first_name?.message}>
        <Input id="first_name" placeholder={t.placeholders.firstName} {...register('first_name')} />
      </FormField>
      <FormField label={t.auth.lastName} htmlFor="last_name" error={errors.last_name?.message}>
        <Input id="last_name" placeholder={t.placeholders.lastName} {...register('last_name')} />
      </FormField>
    </div>
  );
}
