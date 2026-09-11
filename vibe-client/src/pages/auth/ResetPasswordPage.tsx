import { AuthSplitLayout } from '@/shared/components/layout/AuthSplitLayout';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ResetPasswordForm } from './reset-password/ResetPasswordForm';
import { ResetPasswordResult } from './reset-password/ResetPasswordResult';
import { useResetPassword } from './reset-password/useResetPassword';

const t = ES_AR;

export default function ResetPasswordPage() {
  usePageTitle(t.auth.resetPasswordTitle);
  const { status, loading, countdown, onSubmit } = useResetPassword();

  return (
    <AuthSplitLayout>
      <div className="auth-card w-full">
        {status === 'form' ? (
          <>
            <div className="flex flex-col items-center px-0 pb-1 pt-2 md:pt-6">
              <h1 className="font-display text-xl font-bold tracking-tight text-text-primary sm:text-2xl">
                {t.auth.resetPasswordTitle}
              </h1>
              <p className="mt-1 text-center text-sm text-text-tertiary">{t.auth.resetPasswordDescription}</p>
            </div>
            <div className="px-0 pb-6 pt-4">
              <ResetPasswordForm loading={loading} onSubmit={onSubmit} />
            </div>
          </>
        ) : (
          <div className="flex flex-col items-center px-0 py-6">
            <ResetPasswordResult status={status} countdown={countdown} />
          </div>
        )}
      </div>
    </AuthSplitLayout>
  );
}
