import { Phone } from 'lucide-react';
import { OverflowMenu, type OverflowMenuItem } from '@/shared/components/common/OverflowMenu';
import { Tag } from '@/shared/components/common/Tag';
import { WhatsappIcon } from '@/shared/components/common/WhatsappIcon';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useSport } from '@/shared/lib/sportContext';
import { playerTraitsLabel } from '@/shared/lib/traitLabels';
import { cn, formatPrice } from '@/shared/lib/utils';
import { pairFullName, pairPaymentKey, type PairPaymentKey } from '../lib/tournamentView';
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

/** Each player's phone as a call link and a WhatsApp link, in the row's menu. */
function contactItems(pair: TournamentOwnerPair): OverflowMenuItem[] {
  return [pair.payer, pair.partner].flatMap((player) => {
    const name = `${player.first_name} ${player.last_name}`;
    return [
      {
        kind: 'link',
        key: `call-${name}`,
        label: t.call(name),
        icon: <Phone className="size-3.5" aria-hidden="true" />,
        href: `tel:${player.phone}`,
      },
      {
        kind: 'link',
        key: `whatsapp-${name}`,
        label: t.whatsapp(name),
        icon: <WhatsappIcon className="size-3.5" />,
        href: `https://wa.me/${player.phone.replace(/\D/g, '')}`,
        external: true,
      },
    ];
  });
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

/** A phone lists the players one under the other; a wider screen joins them with a slash. */
function PairNames({ pair }: { pair: TournamentOwnerPair }) {
  return (
    <p className="text-text-primary text-sm font-medium" title={pairFullName(pair)}>
      {pair.players.map((player, index) => (
        <span key={`${player.first_name}-${player.last_name}-${String(index)}`} className="block truncate sm:inline">
          {index > 0 && <span className="hidden sm:inline"> / </span>}
          {player.first_name} {player.last_name}
        </span>
      ))}
    </p>
  );
}

function PairRow({
  pair,
  price,
  groupName,
  onSwap,
  onWithdraw,
}: {
  pair: TournamentOwnerPair;
  price: number;
  groupName?: string;
  onSwap?: () => void;
  onWithdraw?: () => void;
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

  const detailLine = details.join(' · ');
  const swapItems: OverflowMenuItem[] = onSwap
    ? [
        { kind: 'action', key: 'swap', label: t.swap, onClick: onSwap },
        { kind: 'separator', key: 'swap-separator' },
      ]
    : [];
  const withdrawItems: OverflowMenuItem[] = onWithdraw
    ? [
        { kind: 'action', key: 'withdraw', label: t.withdraw, onClick: onWithdraw },
        { kind: 'separator', key: 'withdraw-separator' },
      ]
    : [];
  const menuItems = [...swapItems, ...withdrawItems, ...contactItems(pair)];

  // Flat on every width: the names on the left, the badge and the "⋯" menu on the right, on one row.
  return (
    <li className="border-border-subtle flex items-center justify-between gap-3 border-b py-3 last:border-b-0">
      <div className="min-w-0 flex-1 space-y-0.5">
        <PairNames pair={pair} />
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
        <OverflowMenu label={t.actions} items={menuItems} />
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
  /** Whether the owner can take pairs out now: the category is live. */
  canWithdraw?: boolean;
  onWithdraw?: (pairId: string) => void;
  /** The category's price per pair: a pair confirmed at it shows no payment badge. */
  price: number;
  /** A draw exists, so the rows follow the seed order. */
  seeded: boolean;
}

/** Every pair that held a spot: its players, zone, seed and traits, the row's menu (swap, withdraw and contacts), and the payment only when it is an exception. */
export function PairsSection({
  pairs,
  groupNames,
  canSwap,
  onSwap,
  canWithdraw = false,
  onWithdraw,
  price,
  seeded,
}: PairsSectionProps) {
  if (pairs.length === 0) return <p className="text-text-tertiary text-xs">{t.none}</p>;
  const rows = seeded ? [...pairs].sort(bySeed) : pairs;
  return (
    <ul aria-label={ES_AR.tournaments.detail.sections.pairs}>
      {rows.map((pair) => {
        const swappable = canSwap && pair.seed !== null && pair.status === 'active';
        // Only a spot still held or paid can be ended.
        const withdrawable =
          canWithdraw &&
          !!onWithdraw &&
          (pair.registration.status === 'held' || pair.registration.status === 'confirmed');
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
            {...(withdrawable
              ? {
                  onWithdraw: () => {
                    onWithdraw(pair.id);
                  },
                }
              : {})}
          />
        );
      })}
    </ul>
  );
}
