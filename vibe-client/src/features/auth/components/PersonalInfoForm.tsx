import { useForm, Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useStore } from '@/shared/stores';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { PhoneInput } from '@/shared/components/common/PhoneInput';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';
import { useUpdateProfile } from '../hooks/useUpdateProfile';
import { personalInfoSchema, type PersonalInfoFormData } from './personal-info-form/schema';
import { PersonalInfoNameFields } from './personal-info-form/PersonalInfoNameFields';

const t = ES_AR;

export function PersonalInfoForm() {
  const { user } = useStore();

  const form = useForm<PersonalInfoFormData>({
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
      <div className="space-y-1.5">
        <Label htmlFor="email" className="text-xs">
          {t.auth.email}
        </Label>
        <Input id="email" type="email" placeholder={t.placeholders.email} {...form.register('email')} />
        {form.formState.errors.email && (
          <p className="text-xs text-error-text">{form.formState.errors.email.message}</p>
        )}
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="phone" className="text-xs">
          {t.auth.phone}
        </Label>
        <Controller
          name="phone"
          control={form.control}
          render={({ field }) => (
            // Same 288px as the complex's phone field. A phone number is a
            // known length, and two of them at different widths on two screens
            // of the same app says the two expect different answers.
            <div className="max-w-72">
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
        {form.formState.errors.phone && (
          <p className="text-xs text-error-text">{form.formState.errors.phone.message}</p>
        )}
      </div>
      <SectionFooter submitLabel={t.common.save} pending={mutation.isPending} align="start" />
    </form>
  );
}
