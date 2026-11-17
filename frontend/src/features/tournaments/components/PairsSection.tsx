import { Fragment } from 'react';
import { Phone } from 'lucide-react';
import { Tag } from '@/shared/components/common/Tag';
import { WhatsappIcon } from '@/shared/components/common/WhatsappIcon';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useSport } from '@/shared/lib/sportContext';
import { playerTraitsLabel } from '@/shared/lib/traitLabels';
import { cn, formatPrice } from '@/shared/lib/utils';
import { pairFullName, pairName, pairPaymentKey, type PairPaymentKey } from '../lib/tournamentView';
import type { TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.pairs;

// Same colours as the booking payment statuses: amber is money still moving,
// green is paid, grey is a spot that never paid or was refunded.
const STATE_BADGES: Record<PairPaymentKey, { tone: 'success' | 'neutral'; className?: string }> = {
  held: { tone: 'neutral', className: 'bg-warning-bg text-warning-text' },
  confirmed: { tone: 'success' },
  cancelledUnpaid: { tone: 'neutral' },
  refunded: { tone: 'neutral' },
  refundPending: { tone: 'neutral', className: 'bg-warning-bg text-warning-text' },
  partialRefund: { tone: 'neutral', className: 'bg-warning-bg text-warning-text' },
};

/** Both players' phones behind one icon: each number as a tel: link and as a WhatsApp link. */
function ContactMenu({ pair }: { pair: TournamentOwnerPair }) {
  const players = [pair.payer, pair.partner].map((player) => ({
    name: `${player.first_name} ${player.last_name}`,
    phone: player.phone,
  }));
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={t.contact}>
          <Phone className="size-4" aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {players.map((player, index) => (
          <Fragment key={player.name}>
            {index > 0 && <DropdownMenuSeparator />}
            <DropdownMenuLabel>{player.name}</DropdownMenuLabel>
            <DropdownMenuItem asChild>
              <a href={`tel:${player.phone}`} aria-label={`${t.call} ${player.name}`}>
                <Phone className="size-3.5" aria-hidden="true" />
                {player.phone}
              </a>
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <a
                href={`https://wa.me/${player.phone.replace(/\D/g, '')}`}
                target="_blank"
                rel="noopener noreferrer"
                aria-label={t.whatsappAria(player.name)}
              >
                <WhatsappIcon className="size-3.5" />
                {t.whatsapp}
              </a>
            </DropdownMenuItem>
          </Fragment>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * The payment badge only where the payment is an exception: a pair confirmed at
 * the category price, with nothing refunded, shows none.
 */
function paymentBadge(pair: TournamentOwnerPair, price: number) {
  const key = pairPaymentKey(pair);
  const paid = key === 'confirmed' ? (pair.payment?.amount ?? null) : null;
  if (key === 'confirmed' && paid === price && !pair.payment?.refund_amount) return null;
  return { text: paid === null ? t.state[key] : t.paid(formatPrice(paid)), ...STATE_BADGES[key] };
}

/** What came back, on the detail line: the badge already names the payment state. */
function refundedLine(pair: TournamentOwnerPair): string | null {
  const refund = pair.payment?.refund_amount ?? 0;
  return refund > 0 ? t.refundedAmount(formatPrice(refund)) : null;
}

function PairRow({
  pair,
  price,
  groupName,
  onSwap,
}: {
  pair: TournamentOwnerPair;
  price: number;
  groupName?: string;
  onSwap?: () => void;
}) {
  const sport = useSport();
  const badge = paymentBadge(pair, price);
  const traits = [pair.payer, pair.partner]
    .map((player) => playerTraitsLabel({ ...player, sport }))
    .filter((label): label is string => !!label);
  const details = [
    groupName ? t.group(groupName) : null,
    pair.seed !== null ? t.seed(pair.seed) : null,
    pair.status === 'withdrawn' ? t.withdrawn : null,
    traits.length > 0 ? traits.join(' / ') : null,
    refundedLine(pair),
  ].filter((detail): detail is string => !!detail);

  const fullName = pairFullName(pair);
  const detailLine = details.join(' · ');

  // Flat on every width: a phone stacks the names over the actions, a wider screen puts them side by side.
  return (
    <li className="border-border-subtle flex flex-col gap-2 border-b py-3 last:border-b-0 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
      <div className="min-w-0 space-y-0.5 sm:flex-1">
        <p className="text-text-primary truncate text-sm font-medium" title={fullName}>
          {fullName}
        </p>
        {details.length > 0 && (
          <p className="text-text-tertiary truncate text-xs" title={detailLine}>
            {detailLine}
          </p>
        )}
      </div>
      <div className="flex shrink-0 items-center justify-end gap-1.5">
        {badge && (
          <Tag tone={badge.tone} className={cn('text-xs whitespace-nowrap', badge.className)}>
            {badge.text}
          </Tag>
        )}
        <ContactMenu pair={pair} />
        {onSwap && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="rounded-lg whitespace-nowrap"
            aria-label={`${t.swap} ${pairName(pair)}`}
            onClick={onSwap}
          >
            {t.swap}
          </Button>
        )}
      </div>
    </li>
  );
}

/** Seeds in order, the pairs without one after them in their own order. */
function bySeed(a: TournamentOwnerPair, b: TournamentOwnerPair): number {
  return (a.seed ?? Number.MAX_SAFE_INTEGER) - (b.seed ?? Number.MAX_SAFE_INTEGER);
}

interface PairsSectionProps {
  pairs: TournamentOwnerPair[];
  groupNames: Map<string, string>;
  /** Whether pairs can be swapped now: a draw exists and no result is loaded. */
  canSwap: boolean;
  onSwap: (pairId: string) => void;
  /** The category's price per pair: a pair confirmed at it shows no payment badge. */
  price: number;
  /** A draw exists, so the rows follow the seed order. */
  seeded: boolean;
}

/** Every pair that held a spot: its players, zone, seed and traits, the contact menu, and the payment only when it is an exception. */
export function PairsSection({ pairs, groupNames, canSwap, onSwap, price, seeded }: PairsSectionProps) {
  if (pairs.length === 0) return <p className="text-text-tertiary text-xs">{t.none}</p>;
  const rows = seeded ? [...pairs].sort(bySeed) : pairs;
  return (
    <ul aria-label={ES_AR.tournaments.detail.sections.pairs}>
      {rows.map((pair) => {
        const swappable = canSwap && pair.seed !== null && pair.status === 'active';
        const groupName = pair.group_id ? groupNames.get(pair.group_id) : undefined;
        return (
          <PairRow
            key={pair.id}
            pair={pair}
            price={price}
            {...(groupName ? { groupName } : {})}
            {...(swappable
              ? {
                  onSwap: () => {
                    onSwap(pair.id);
                  },
                }
              : {})}
          />
        );
      })}
    </ul>
  );
}
