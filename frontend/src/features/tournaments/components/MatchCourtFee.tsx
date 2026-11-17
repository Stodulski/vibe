import { ES_AR } from '@/shared/i18n/es_AR';
import { COUNTER_PAYMENT_METHODS } from '@/shared/lib/paymentMethods';
import { formatPrice } from '@/shared/lib/utils';
import { courtFeeSides, slotLabel } from '../lib/tournamentView';
import type { CourtFeeTarget } from '../hooks/useTournamentChanges';
import type { TournamentCourtFee, TournamentOwnerMatch, TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.courtFee;

/** Charging and undoing a pair's court fee: the match's overflow menu holds them, never the row. */
export interface CourtFeeActions {
  onCharge: (target: CourtFeeTarget) => void;
  onUndo: (target: CourtFeeTarget) => void;
}

interface MatchCourtFeeProps {
  match: TournamentOwnerMatch;
  /** The category's current fee per pair, centavos. */
  fee: number;
  pairs: Map<string, TournamentOwnerPair>;
  groupNames: Map<string, string>;
}

/** The method a charge was made with; one this build does not know is shown as the server names it. */
function methodLabel(method: string): string {
  const known = COUNTER_PAYMENT_METHODS.find((m) => m === method);
  return known ? ES_AR.bookings.paymentMethods[known] : method;
}

/** A pair's court fee as a status line: what it paid and how, or that it is still pending. */
function FeeRow({ name, side }: { name: string; side: TournamentCourtFee }) {
  return (
    <li className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-0.5 text-xs">
      <span className="text-text-secondary max-w-full min-w-0 truncate" title={name}>
        {name}
      </span>
      {side.charged ? (
        <span className="flex min-w-0 shrink-0 items-baseline gap-2">
          <span className="text-text-primary whitespace-nowrap">{t.paid(formatPrice(side.amount))}</span>
          {side.method && (
            <span className="text-text-tertiary min-w-0 truncate" title={methodLabel(side.method)}>
              {methodLabel(side.method)}
            </span>
          )}
        </span>
      ) : (
        <span className="text-text-tertiary shrink-0">{ES_AR.tournaments.matches.pending}</span>
      )}
    </li>
  );
}

/**
 * What the match's court costs each pair and where each one stands: paid at the
 * amount it was charged, with its method, or still pending. A pair that owes
 * nothing for the match (not known yet, a walkover, a bye) is not listed.
 */
export function MatchCourtFee({ match, fee, pairs, groupNames }: MatchCourtFeeProps) {
  const rows = courtFeeSides(match);
  if (fee <= 0 && rows.length === 0) return null;
  return (
    <div className="space-y-1.5">
      {fee > 0 && (
        <p className="text-text-secondary truncate text-xs" title={t.line(formatPrice(fee))}>
          {t.line(formatPrice(fee))}
        </p>
      )}
      {rows.length > 0 && (
        <ul className="space-y-1">
          {rows.map((side) => (
            <FeeRow key={side.side} name={slotLabel(match, side.side, pairs, groupNames)} side={side} />
          ))}
        </ul>
      )}
    </div>
  );
}
