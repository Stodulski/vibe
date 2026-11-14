import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COUNTER_PAYMENT_METHODS } from '@/shared/lib/paymentMethods';
import { formatPrice } from '@/shared/lib/utils';
import { slotLabel } from '../lib/tournamentView';
import type { CourtFeeTarget } from '../hooks/useTournamentChanges';
import type { TournamentCourtFee, TournamentOwnerMatch, TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.courtFee;

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
  actions: CourtFeeActions;
  editable: boolean;
}

/** The method a charge was made with; one this build does not know is shown as the server names it. */
function methodLabel(method: string): string {
  const known = COUNTER_PAYMENT_METHODS.find((m) => m === method);
  return known ? ES_AR.bookings.paymentMethods[known] : method;
}

function FeeRow({
  name,
  side,
  match,
  actions,
  editable,
}: {
  name: string;
  side: TournamentCourtFee;
  match: TournamentOwnerMatch;
  actions: CourtFeeActions;
  editable: boolean;
}) {
  const target = { matchId: match.id, side: side.side };
  return (
    <li className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
      <span className="text-text-secondary min-w-0 break-words">{name}</span>
      {side.charged ? (
        <>
          <span className="text-text-primary score-text font-medium">{t.paid(formatPrice(side.amount))}</span>
          {side.method && <span className="text-text-tertiary text-xs">{methodLabel(side.method)}</span>}
          {editable && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="rounded-lg"
              aria-label={t.undoAria(name)}
              onClick={() => {
                actions.onUndo(target);
              }}
            >
              {t.undo}
            </Button>
          )}
        </>
      ) : (
        editable && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="rounded-lg"
            aria-label={t.chargeAria(name)}
            onClick={() => {
              actions.onCharge(target);
            }}
          >
            {t.charge}
          </Button>
        )
      )}
    </li>
  );
}

/**
 * What the match's court costs each pair, and per pair either to charge it or
 * that it paid (at the amount it was charged) with a way to undo it. A pair that
 * owes nothing for the match (not known yet, a walkover, a bye) is not listed.
 */
export function MatchCourtFee({ match, fee, pairs, groupNames, actions, editable }: MatchCourtFeeProps) {
  const rows = match.court_fee.filter((side) => side.charged || side.chargeable);
  if (fee <= 0 && rows.length === 0) return null;
  return (
    <div className="space-y-1.5">
      {fee > 0 && <p className="text-text-secondary text-xs">{t.line(formatPrice(fee))}</p>}
      {rows.length > 0 && (
        <ul className="space-y-1">
          {rows.map((side) => (
            <FeeRow
              key={side.side}
              name={slotLabel(match, side.side, pairs, groupNames)}
              side={side}
              match={match}
              actions={actions}
              editable={editable}
            />
          ))}
        </ul>
      )}
    </div>
  );
}
