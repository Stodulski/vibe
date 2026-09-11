import { memo } from 'react';
import { Ban } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { TappableCard } from '@/shared/components/common/TappableCard';
import { ClientActionsMenu } from '../ClientActionsMenu';
import { attendancePct, attendanceTone } from './attendance';
import type { Client } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * One client, as a card that opens their detail.
 *
 * The body opens the detail; the menu in the corner offers the three things
 * worth doing without opening it — WhatsApp, call, block. It is the same
 * component the detail's own header uses, so the two cannot come to offer
 * different actions.
 *
 * Blocked is still shown, because that is a state and not an action: the card
 * dims and says so. What went is the ability to block from here.
 */
export const ClientCard = memo(function ClientCard({
  client,
  onSelect,
  onBlock,
}: {
  client: Client;
  onSelect: (client: Client) => void;
  onBlock: (client: Client) => void;
}) {
  const fullName = `${client.first_name} ${client.last_name}`;
  const pct = attendancePct(client);
  const tone = attendanceTone(pct);

  return (
    // The menu is a SIBLING of the tappable button, never a descendant: an
    // interactive element inside a <button> is invalid HTML and a screen
    // reader cannot reach it.
    <TappableCard
      onTap={() => {
        onSelect(client);
      }}
      buttonLabel={fullName}
      className={cn(client.is_blocked && 'opacity-60')}
      actionClassName="absolute right-3 top-3"
      action={<ClientActionsMenu client={client} onBlock={onBlock} />}
      buttonClassName={cn(
        'focus-self flex w-full flex-col items-stretch rounded-2xl border border-border-subtle',
        'bg-bg-subtle p-4 text-left transition-colors hover:border-border-default hover:bg-bg-elevated',
        'focus-visible:border-primary-400 focus-visible:bg-bg-elevated',
      )}
    >
      {/* Padded on the right so the name never runs under the menu. */}
      <div className="flex items-start gap-2 pr-16">
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-text-primary">{fullName}</p>
          <p className="score-text mt-0.5 truncate text-xs text-text-tertiary">{client.phone}</p>
        </div>
        {client.is_blocked && <Ban className="mt-0.5 size-4 shrink-0 text-error-text" aria-label={t.clients.blocked} />}
      </div>

      {/* Two figures, each under its own label. The numbers line up across the
          grid so a column of cards can be read down rather than one at a time. */}
      <div className="mt-4 grid grid-cols-2 gap-3">
        <Figure label={t.clients.bookingsLabel}>
          <span className="score-text text-lg font-bold text-text-primary">{client.total_bookings}</span>
        </Figure>
        <Figure label={t.clients.attendanceLabel}>
          <div className="flex items-center gap-2">
            <span className={cn('score-text text-sm font-bold', tone.text)}>{pct}%</span>
            <div className="h-1 min-w-0 flex-1 overflow-hidden rounded-full bg-bg-base">
              <div className={cn('h-full rounded-full', tone.bar)} style={{ width: `${String(pct)}%` }} />
            </div>
          </div>
        </Figure>
      </div>
    </TappableCard>
  );
});

function Figure({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="min-w-0">
      <p className="text-micro font-medium uppercase tracking-wider text-text-tertiary">{label}</p>
      <div className="mt-1">{children}</div>
    </div>
  );
}
