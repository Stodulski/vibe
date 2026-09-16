import { AuthSplitLayout } from '@/shared/components/layout/AuthSplitLayout';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ConfirmEmailChangeStatus } from './confirm-email-change/ConfirmEmailChangeStatus';
import { useConfirmEmailChange } from '@/features/auth/hooks/useConfirmEmailChange';

export default function ConfirmEmailChangePage() {
  usePageTitle('Confirmar cambio de email');
  const { status, confirm } = useConfirmEmailChange();

  return (
    <AuthSplitLayout>
      <div className="auth-card w-full">
        <div className="flex flex-col items-center px-0 py-6">
          <ConfirmEmailChangeStatus status={status} onConfirm={confirm} />
        </div>
      </div>
    </AuthSplitLayout>
  );
}
