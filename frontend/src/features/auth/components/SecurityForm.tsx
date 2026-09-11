import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { passwordField } from '@/shared/lib/validations';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { DeleteAccountSection } from './DeleteAccountSection';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';
import { useChangePassword } from '../hooks/useChangePassword';

const t = ES_AR;

const schema = z
  .object({
    current_password: z.string().min(1, t.common.required),
    new_password: passwordField,
    confirm_password: z.string().min(1, t.common.required),
  })
  .refine((d) => d.new_password === d.confirm_password, {
    message: t.auth.passwordMismatch,
    path: ['confirm_password'],
  });

type FormData = z.infer<typeof schema>;

export function SecurityForm() {
  const form = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { current_password: '', new_password: '', confirm_password: '' },
  });

  const mutation = useChangePassword();

  return (
    <form
      onSubmit={submitHandler(form.handleSubmit, (d) => {
        mutation.mutate(d);
      })}
      className="max-w-xl space-y-4"
    >
      <div className="space-y-1.5">
        <Label htmlFor="current_password" className="text-xs">
          {t.auth.currentPassword}
        </Label>
        <Input id="current_password" type="password" {...form.register('current_password')} />
        {form.formState.errors.current_password && (
          <p className="text-xs text-error-text">{form.formState.errors.current_password.message}</p>
        )}
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="new_password" className="text-xs">
            {t.auth.newPassword}
          </Label>
          <Input id="new_password" type="password" {...form.register('new_password')} />
          {form.formState.errors.new_password && (
            <p className="text-xs text-error-text">{form.formState.errors.new_password.message}</p>
          )}
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="confirm_password" className="text-xs">
            {t.auth.confirmNewPassword}
          </Label>
          <Input id="confirm_password" type="password" {...form.register('confirm_password')} />
          {form.formState.errors.confirm_password && (
            <p className="text-xs text-error-text">{form.formState.errors.confirm_password.message}</p>
          )}
        </div>
      </div>
      <p className="text-micro text-text-tertiary">{t.profile.passwordHint}</p>
      <SectionFooter
        submitLabel={t.auth.changePassword}
        pending={mutation.isPending}
        align="start"
        extra={<DeleteAccountSection />}
      />
    </form>
  );
}
