import { Link } from 'react-router-dom';
import { CheckCircle2 } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function ForgotPasswordSuccess() {
  return (
    <div className="flex flex-col items-center py-4">
      <div className="mb-4 rounded-2xl bg-success-bg p-3 ring-1 ring-success-border">
        <CheckCircle2 className="size-8 text-success-icon" />
      </div>
      <p className="text-center text-sm text-text-secondary">{t.auth.forgotPasswordSuccess}</p>
      <Link
        to="/login"
        className="mt-6 inline-flex h-11 items-center justify-center rounded-full bg-primary px-8 text-sm font-semibold text-primary-foreground transition-colors hover:brightness-110 focus-visible:ring-[3px] focus-visible:ring-ring/50"
      >
        {t.auth.goToLogin}
      </Link>
    </div>
  );
}
