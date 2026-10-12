import { Mail } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface GoogleCompleteEmailFieldProps {
  email: string;
}

/** The Google account's email — read-only, it isn't part of the form's own validated fields. */
export function GoogleCompleteEmailField({ email }: GoogleCompleteEmailFieldProps) {
  return (
    <div className="space-y-1.5">
      <FormField label={t.auth.email} htmlFor="google-email" icon={Mail}>
        <Input
          id="google-email"
          type="email"
          value={email}
          readOnly
          disabled
          autoComplete="email"
          aria-describedby="google-email-notice"
          className="h-11 pl-9 text-sm sm:h-10"
        />
      </FormField>
      {/* Two block lines, not one paragraph, for the same reason as the register notice:
          each line is its own LCP text candidate and stays under the brand mark's size. */}
      <div id="google-email-notice" className="text-micro text-text-tertiary">
        <p>{t.auth.abandonedLeadNoticeFirst}</p> <p>{t.auth.abandonedLeadNoticeSecond}</p>
      </div>
    </div>
  );
}
