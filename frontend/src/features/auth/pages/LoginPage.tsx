import { AlertCircle } from 'lucide-react';
import { LoginForm } from '@/features/auth';
import { AuthSplitLayout } from '@/shared/components/layout/AuthSplitLayout';
import { usePageTitle, useOGTags } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useGoogleLoginError } from './login/useGoogleLoginError';

const t = ES_AR;

export default function LoginPage() {
  usePageTitle(t.auth.login);
  useOGTags({
    title: t.auth.loginMetaTitle,
    description: t.auth.loginMetaDescription,
  });
  const googleError = useGoogleLoginError();

  return (
    <AuthSplitLayout>
      {/* Inline, above the card, rather than a toast: a Google sign-in that
          failed sent the person here through a full page load, and a toast
          that auto-dismisses (or that a re-render drops) leaves them staring
          at a login form with no idea why they are back on it. */}
      {googleError && (
        <div
          role="alert"
          className="border-error-border/30 bg-error-bg text-error-text mb-4 flex items-start gap-2 rounded-xl border p-3 text-sm"
        >
          <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          {googleError}
        </div>
      )}
      <LoginForm />
    </AuthSplitLayout>
  );
}
