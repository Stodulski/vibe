import { CheckCircle2, Unplug } from 'lucide-react';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function StatusIndicator({ connected, mpUserId }: { connected: boolean; mpUserId: string | undefined }) {
  return (
    // No container. A bordered box around a line of state read as a card you
    // could act on, next to a button that actually is one. The icon and the
    // colour carry the status; the box was only holding them.
    <div className="flex items-center gap-2.5">
      <div
        className={cn(
          'flex size-8 shrink-0 items-center justify-center rounded-full',
          connected ? 'bg-success-bg text-success-text' : 'bg-bg-elevated text-text-tertiary',
        )}
      >
        {connected ? <CheckCircle2 className="size-4" /> : <Unplug className="size-4" />}
      </div>
      <div className="min-w-0">
        <p className={cn('text-sm font-semibold', connected ? 'text-success-text' : 'text-text-primary')}>
          {connected ? t.mp.connected : t.mp.notConnected}
        </p>
        {connected && mpUserId && <p className="score-text text-micro text-text-tertiary">ID: {mpUserId}</p>}
      </div>
    </div>
  );
}
