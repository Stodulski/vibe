import { AlertCircle, Loader2 } from 'lucide-react';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { StatusIndicator } from './mp-connect-card/StatusIndicator';
import { InfoSection } from './mp-connect-card/InfoSection';
import { ConnectAction } from './mp-connect-card/ConnectAction';
import { useMPConnect } from './mp-connect-card/useMPConnect';
import { MPFeesSection } from './mp-connect-card/MPFeesSection';

const t = ES_AR;

interface MPConnectCardProps {
  complexId: string;
  province: string;
}

// Without this, a failed `mp/status` left `mpStatus` `undefined`, which reads
// exactly like "not connected" (`connected: mpStatus?.connected ?? false`) —
// the owner sees a disabled "Conectar" button forever and no indication that
// anything actually went wrong.
function MPConnectCardError({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-8">
      <AlertCircle className="size-8 text-text-tertiary" />
      <p className="text-sm text-text-tertiary">{t.common.error}</p>
      <Button variant="outline" size="sm" onClick={onRetry}>
        {t.common.refresh}
      </Button>
    </div>
  );
}

// The `mp/status` loading/error/connected states, split out so `MPConnectCard`
// can always render `MPFeesSection` underneath regardless of which one is
// active — the owner deciding how to accredit money doesn't require having
// connected an account, or `mp/status` having answered, yet.
function MPConnectCardStatus({ complexId }: { complexId: string }) {
  const {
    isLoading,
    isError,
    refetch,
    connected,
    mpUserId,
    authUrl,
    handleConnectClick,
    showDisconnect,
    setShowDisconnect,
    disconnectMutation,
  } = useMPConnect(complexId);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="size-5 animate-spin text-primary-400" />
      </div>
    );
  }

  if (isError) {
    return (
      <MPConnectCardError
        onRetry={() => {
          void refetch();
        }}
      />
    );
  }

  return (
    <>
      <StatusIndicator connected={connected} mpUserId={mpUserId} />
      <InfoSection connected={connected} />
      <ConnectAction
        connected={connected}
        authUrl={authUrl}
        onConnectClick={handleConnectClick}
        onDisconnectClick={() => {
          setShowDisconnect(true);
        }}
      />

      <ConfirmDialog
        open={showDisconnect}
        onClose={() => {
          setShowDisconnect(false);
        }}
        title={t.mp.disconnect}
        description={t.mp.disconnectConfirm}
        onConfirm={() => {
          disconnectMutation.mutate();
        }}
        isLoading={disconnectMutation.isPending}
        variant="destructive"
      />
    </>
  );
}

export function MPConnectCard({ complexId, province }: MPConnectCardProps) {
  return (
    <div className="space-y-5">
      <MPConnectCardStatus complexId={complexId} />
      <MPFeesSection province={province} />
    </div>
  );
}
