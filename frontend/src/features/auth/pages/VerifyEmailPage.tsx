import { AuthSplitLayout } from '@/shared/components/layout/AuthSplitLayout';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { VerifyEmailStatus } from './verify-email/VerifyEmailStatus';
import { useVerifyEmail } from '@/features/auth/hooks/useVerifyEmail';

export default function VerifyEmailPage() {
  usePageTitle('Verificar email');
  const { status, countdown } = useVerifyEmail();

  return (
    <AuthSplitLayout>
      <div className="auth-card w-full">
        <div className="flex flex-col items-center px-0 py-6">
          <VerifyEmailStatus status={status} countdown={countdown} />
        </div>
      </div>
    </AuthSplitLayout>
  );
}
