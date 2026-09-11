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
    <FormField label={t.auth.email} htmlFor="google-email" icon={Mail}>
      <Input
        id="google-email"
        type="email"
        value={email}
        readOnly
        disabled
        autoComplete="email"
        className="h-11 pl-9 text-sm sm:h-10"
      />
    </FormField>
  );
}
