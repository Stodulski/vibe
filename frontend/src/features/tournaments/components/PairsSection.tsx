import { ES_AR } from '@/shared/i18n/es_AR';
import { cn, formatPrice } from '@/shared/lib/utils';
import { pairFullName, pairName, pairPaymentKey, type PairPaymentKey } from '../lib/tournamentView';
import { Button } from '@/shared/components/ui/button';
import type { TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.pairs;

// Same colours as the booking payment statuses: amber is money still moving,
// green is paid, grey is a spot that never paid or was refunded.
const STATE_STYLES: Record<PairPaymentKey, string> = {
  held: 'text-warning-text',
  confirmed: 'text-success-text',
  cancelledUnpaid: 'text-text-secondary',
  refunded: 'text-text-secondary',
  refundPending: 'text-warning-text',
  partialRefund: 'text-warning-text',
};

function PhoneLink({ name, phone }: { name: string; phone: string }) {
  return (
    <a
      href={`tel:${phone}`}
      aria-label={`${t.call} ${name}`}
      className="inline-link text-text-tertiary score-text text-xs break-all hover:underline"
    >
      {phone}
    </a>
  );
}

// A confirmed pair with a partial refund (made from Mercado Pago's panel) still
// holds the spot, so the line keeps what they paid next to what came back.
function moneyLine(pair: TournamentOwnerPair): string | null {
  const payment = pair.payment;
  if (!payment) return null;
  const paid = pair.registration.status === 'confirmed' ? t.paid(formatPrice(payment.amount)) : null;
  const refunded = payment.refund_amount > 0 ? t.refundedAmount(formatPrice(payment.refund_amount)) : null;
  const parts = [paid, refunded].filter(Boolean);
  return parts.length > 0 ? parts.join(' · ') : null;
}

function PairRow({ pair, groupName, onSwap }: { pair: TournamentOwnerPair; groupName?: string; onSwap?: () => void }) {
  const key = pairPaymentKey(pair);
  const tags = [
    pair.seed !== null ? t.seed(pair.seed) : null,
    groupName ? t.group(groupName) : null,
    pair.status === 'withdrawn' ? t.withdrawn : null,
    moneyLine(pair),
  ].filter(Boolean);

  return (
    <li className="border-border-subtle flex items-baseline justify-between gap-3 border-b py-3 last:border-b-0">
      <div className="min-w-0 space-y-0.5">
        <p className="text-text-primary text-sm font-medium break-words">{pairFullName(pair)}</p>
        <div className="flex flex-wrap gap-x-3 gap-y-0.5">
          <PhoneLink name={`${pair.payer.first_name} ${pair.payer.last_name}`} phone={pair.payer.phone} />
          <PhoneLink name={`${pair.partner.first_name} ${pair.partner.last_name}`} phone={pair.partner.phone} />
        </div>
        {tags.length > 0 && <p className="text-text-tertiary text-xs">{tags.join(' · ')}</p>}
      </div>
      <div className="flex shrink-0 flex-col items-end gap-1.5">
        <span className={cn('text-xs font-medium', STATE_STYLES[key])}>{t.state[key]}</span>
        {onSwap && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="rounded-lg"
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

interface PairsSectionProps {
  pairs: TournamentOwnerPair[];
  groupNames: Map<string, string>;
  /** Whether pairs can be swapped now: a draw exists and no result is loaded. */
  canSwap: boolean;
  onSwap: (pairId: string) => void;
}

/** Every pair that held a spot: players, contact, seed and group once drawn, and the payment behind it. */
export function PairsSection({ pairs, groupNames, canSwap, onSwap }: PairsSectionProps) {
  if (pairs.length === 0) return <p className="text-text-tertiary text-sm">{t.none}</p>;
  return (
    <ul aria-label={ES_AR.tournaments.detail.sections.pairs}>
      {pairs.map((pair) => {
        const swappable = canSwap && pair.seed !== null && pair.status === 'active';
        const groupName = pair.group_id ? groupNames.get(pair.group_id) : undefined;
        return (
          <PairRow
            key={pair.id}
            pair={pair}
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
