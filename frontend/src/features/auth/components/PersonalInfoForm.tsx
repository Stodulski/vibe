import { Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Input } from '@/shared/components/ui/input';
import { PhoneInput } from '@/shared/components/common/PhoneInput';
import { FormField } from '@/shared/components/common/FormField';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useAppForm, submitHandler } from '@/shared/lib/form';
import { useAuth } from '../hooks/useAuth';
import { useUpdateProfile } from '../hooks/useUpdateProfile';
import { personalInfoSchema, type PersonalInfoFormData } from './personal-info-form/personalInfo.schema';
import { PersonalInfoNameFields } from './personal-info-form/PersonalInfoNameFields';

const t = ES_AR;

export function PersonalInfoForm() {
  const { user, pendingEmail } = useAuth();

  const form = useAppForm<PersonalInfoFormData>({
    resolver: zodResolver(personalInfoSchema),
    defaultValues: {
      first_name: user?.first_name ?? '',
      last_name: user?.last_name ?? '',
      email: user?.email ?? '',
      phone: user?.phone ?? '',
    },
  });

  const mutation = useUpdateProfile();

  return (
    <form
      onSubmit={submitHandler(form.handleSubmit, (d) => {
        mutation.mutate(d);
      })}
      className="max-w-xl space-y-4"
    >
      <PersonalInfoNameFields register={form.register} errors={form.formState.errors} />
      <FormField
        label={t.auth.email}
        htmlFor="email"
        error={form.formState.errors.email?.message}
        helpText={
          pendingEmail && (
            <>
              {t.auth.emailChangePending} <span className="text-text-secondary font-medium">{pendingEmail}</span>
            </>
          )
        }
      >
        <Input id="email" type="email" placeholder={t.placeholders.email} {...form.register('email')} />
      </FormField>
      <FormField label={t.auth.phone} htmlFor="phone" error={form.formState.errors.phone?.message}>
        <Controller
          name="phone"
          control={form.control}
          render={({ field }) => (
            // Same 288px as the complex's phone field. A phone number is a
            // known length, and two of them at different widths on two screens
            // of the same app says the two expect different answers.
            <div className="sm:max-w-72">
              <PhoneInput
                id="phone"
                value={field.value}
                onChange={field.onChange}
                onBlur={field.onBlur}
                aria-invalid={!!form.formState.errors.phone}
              />
            </div>
          )}
        />
      </FormField>
      <SectionFooter submitLabel={t.common.save} pending={mutation.isPending} align="start" />
    </form>
  );
}
