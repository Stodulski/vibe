import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Mail } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { RegisterDto } from '../../schemas/auth.schema';

const t = ES_AR;

interface RegisterEmailFieldProps {
  register: UseFormRegister<RegisterDto>;
  errors: FieldErrors<RegisterDto>;
}

export function RegisterEmailField({ register, errors }: RegisterEmailFieldProps) {
  return (
    <div className="auth-stagger-1 space-y-1.5">
      <FormField label={t.auth.email} htmlFor="email" icon={Mail} error={errors.email?.message}>
        <Input
          id="email"
          type="email"
          placeholder={t.placeholders.email}
          autoComplete="email"
          aria-invalid={!!errors.email}
          aria-describedby={errors.email ? 'email-error email-notice' : 'email-notice'}
          className="h-11 pl-9 text-sm sm:h-10"
          {...register('email')}
        />
      </FormField>
      {/* Two block lines, not one paragraph. Chrome picks the LCP text per block, so one
          wrapped paragraph of this copy is wider than the brand mark and takes over as the
          largest element on the page. Each line on its own stays under the mark's size. The
          space between the lines is what a screen reader reads as the break; on screen it
          collapses to nothing. */}
      <div id="email-notice" className="text-micro text-text-tertiary">
        <p>{t.auth.abandonedLeadNoticeFirst}</p> <p>{t.auth.abandonedLeadNoticeSecond}</p>
      </div>
    </div>
  );
}
