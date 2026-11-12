import { AlertCircle } from 'lucide-react';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { Button } from '@/shared/components/ui/button';
import { Skeleton } from '@/shared/components/ui/skeleton';
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
      <AlertCircle className="text-text-tertiary size-8" />
      <p className="text-text-tertiary text-sm">{t.common.error}</p>
      <Button variant="outline" size="sm" onClick={onRetry}>
        {t.common.refresh}
      </Button>
    </div>
  );
}

// Holds the status block's height while `mp/status` loads, so the fees table
// below does not drop when it arrives. Mirrors the loaded block: the same
// `space-y-5` rhythm, a 32px icon row, InfoSection's three lines (16/16/14px,
// 12px apart) and a 36px button after 8px of padding. Not connected is the
// layout used because the loaded state is not known yet (about 186px).
function MPConnectCardSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading} className="space-y-5">
      <div className="flex items-center gap-2.5">
        <Skeleton className="size-8 shrink-0 rounded-full" />
        <div className="flex h-5 items-center">
          <Skeleton className="h-4 w-48 rounded-lg" />
        </div>
      </div>
      <div className="space-y-3">
        <div className="flex h-4 items-center">
          <Skeleton className="h-3 w-64 rounded-lg" />
        </div>
        <div className="flex h-4 items-center gap-2">
          <Skeleton className="size-3.5 shrink-0 rounded-full" />
          <Skeleton className="h-3 w-48 rounded-lg" />
        </div>
        <div className="flex h-3.5 items-center">
          <Skeleton className="h-2.5 w-56 rounded-lg" />
        </div>
      </div>
      <div className="pt-2">
        <Skeleton className="h-9 w-full rounded-full sm:w-28" />
      </div>
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
    return <MPConnectCardSkeleton />;
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
