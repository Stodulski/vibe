import { AppHeader } from '@/shared/components/layout/AppHeader';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { MPCallbackStatus } from './mp-callback-page/MPCallbackStatus';
import { useMPCallback } from './mp-callback-page/useMPCallback';

export default function MPCallbackPage() {
  usePageTitle('MercadoPago');
  const { status, returnPath } = useMPCallback();

  return (
    <div className="bg-bg-base flex min-h-dvh flex-col">
      <AppHeader />
      <div className="flex flex-1 items-center justify-center">
        <MPCallbackStatus status={status} returnPath={returnPath} />
      </div>
    </div>
  );
}
