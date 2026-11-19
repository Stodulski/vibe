import { useState } from 'react';
import { AlertCircle, AlertTriangle, CheckCircle2, ExternalLink, Info, Loader2, Unplug } from 'lucide-react';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { useConnectInstagram, useDisconnectInstagram, useInstagramStatus } from '../hooks/useInstagram';
import { InstagramConnectCardSkeleton } from './InstagramConnectCardSkeleton';
import { InstagramStoriesSection } from './instagram-stories/InstagramStoriesSection';

const t = ES_AR;

type ConnectionState = 'connected' | 'needsReconnect' | 'notConnected';

const STATE_TITLE: Record<ConnectionState, string> = {
  connected: t.instagram.connected,
  needsReconnect: t.instagram.needsReconnect,
  notConnected: t.instagram.notConnected,
};

// Same reason as MPConnectCard's: a failed status read must not look like
// "not connected" with a button that then fails too.
function InstagramConnectCardError({ onRetry }: { onRetry: () => void }) {
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

function StatusRow({ state, username }: { state: ConnectionState; username: string | null }) {
  return (
    <div className="flex items-center gap-2.5">
      <div
        className={cn(
          'flex size-8 shrink-0 items-center justify-center rounded-full',
          state === 'connected' && 'bg-success-bg text-success-text',
          state === 'needsReconnect' && 'bg-warning-bg text-warning-text',
          state === 'notConnected' && 'bg-bg-elevated text-text-tertiary',
        )}
      >
        {state === 'connected' && <CheckCircle2 className="size-4" />}
        {state === 'needsReconnect' && <AlertTriangle className="size-4" />}
        {state === 'notConnected' && <Unplug className="size-4" />}
      </div>
      <div className="min-w-0">
        <p
          className={cn(
            'text-sm font-semibold',
            state === 'connected' && 'text-success-text',
            state === 'needsReconnect' && 'text-warning-text',
            state === 'notConnected' && 'text-text-primary',
          )}
        >
          {STATE_TITLE[state]}
        </p>
        {state !== 'notConnected' && username && <p className="text-micro text-text-tertiary truncate">@{username}</p>}
      </div>
    </div>
  );
}

function InfoBlock({ state }: { state: ConnectionState }) {
  return (
    <div className="space-y-3">
      <p className="text-text-secondary text-xs">
        {state === 'needsReconnect' ? t.instagram.needsReconnectDescription : t.instagram.description}
      </p>
      {state !== 'connected' && (
        <p className="text-text-tertiary flex items-start gap-2 text-xs">
          <Info className="mt-px size-3.5 shrink-0" />
          {t.instagram.professionalNote}
        </p>
      )}
    </div>
  );
}

function ConnectionActions({
  state,
  connecting,
  onConnect,
  onDisconnect,
}: {
  state: ConnectionState;
  connecting: boolean;
  onConnect: () => void;
  onDisconnect: () => void;
}) {
  return (
    <>
      {/* The footer lays its children out reversed (stacked on a phone, in a
        row from sm), so Disconnect comes first in the DOM to land after
        Reconnect on screen. */}
      <SectionFooter align="start" className="border-t-0 pt-2">
        {state !== 'notConnected' && (
          <Button
            variant="outline"
            className="border-error-border/30 text-error-text hover:bg-error-bg w-full sm:w-auto"
            onClick={onDisconnect}
          >
            <Unplug className="size-3.5" />
            {t.instagram.disconnect}
          </Button>
        )}
        {state !== 'connected' && (
          <Button variant="outline" className="w-full sm:w-auto" disabled={connecting} onClick={onConnect}>
            {connecting ? <Loader2 className="size-3.5 animate-spin" /> : <ExternalLink className="size-3.5" />}
            {state === 'needsReconnect' ? t.instagram.reconnect : t.instagram.connect}
          </Button>
        )}
      </SectionFooter>
    </>
  );
}

/**
 * Connects the complex's Instagram professional account so Vibe can publish
 * stories with its free slots. Connecting leaves the app for Instagram's
 * consent page; the result comes back in the URL and is shown by the
 * settings page (`useInstagramCallback`).
 */
export function InstagramConnectCard({ complexId, slug }: { complexId: string; slug: string }) {
  const [showDisconnect, setShowDisconnect] = useState(false);
  const { data: status, isLoading, isError, refetch } = useInstagramStatus(complexId);
  const connectMutation = useConnectInstagram(complexId);
  const disconnectMutation = useDisconnectInstagram(complexId, () => {
    setShowDisconnect(false);
  });

  if (isLoading) return <InstagramConnectCardSkeleton />;
  if (isError || !status) {
    return (
      <InstagramConnectCardError
        onRetry={() => {
          void refetch();
        }}
      />
    );
  }

  const state: ConnectionState = status.connected
    ? 'connected'
    : status.needs_reconnect
      ? 'needsReconnect'
      : 'notConnected';
  // A successful connect is followed by a full page navigation, so the
  // button stays busy until the browser leaves instead of re-enabling.
  const connecting = connectMutation.isPending || connectMutation.isSuccess;

  return (
    <div className="space-y-5">
      <StatusRow state={state} username={status.username} />

      <InfoBlock state={state} />

      <ConnectionActions
        state={state}
        connecting={connecting}
        onConnect={() => {
          connectMutation.mutate();
        }}
        onDisconnect={() => {
          setShowDisconnect(true);
        }}
      />

      <ConfirmDialog
        open={showDisconnect}
        onClose={() => {
          setShowDisconnect(false);
        }}
        title={t.instagram.disconnect}
        description={t.instagram.disconnectConfirm}
        onConfirm={() => {
          disconnectMutation.mutate();
        }}
        isLoading={disconnectMutation.isPending}
        variant="destructive"
      />

      {/* Stories need a usable connection: nothing to configure without one. */}
      {state === 'connected' && !status.needs_reconnect && (
        <InstagramStoriesSection complexId={complexId} slug={slug} />
      )}
    </div>
  );
}
