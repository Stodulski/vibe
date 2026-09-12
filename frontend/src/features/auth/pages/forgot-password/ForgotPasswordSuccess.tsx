import { Link } from 'react-router-dom';
import { CheckCircle2 } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function ForgotPasswordSuccess() {
  return (
    <div className="flex flex-col items-center py-4">
      <div className="bg-success-bg ring-success-border mb-4 rounded-2xl p-3 ring-1">
        <CheckCircle2 className="text-success-icon size-8" />
      </div>
      <p className="text-text-secondary text-center text-sm">{t.auth.forgotPasswordSuccess}</p>
      <Link
        to="/login"
        className="bg-primary text-primary-foreground focus-visible:ring-ring/50 mt-6 inline-flex h-11 items-center justify-center rounded-full px-8 text-sm font-semibold transition-colors hover:brightness-110 focus-visible:ring-[3px]"
      >
        {t.auth.goToLogin}
      </Link>
    </div>
  );
}
