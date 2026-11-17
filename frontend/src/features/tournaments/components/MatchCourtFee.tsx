import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COUNTER_PAYMENT_METHODS } from '@/shared/lib/paymentMethods';
import { formatPrice } from '@/shared/lib/utils';
import { slotLabel } from '../lib/tournamentView';
import type { CourtFeeTarget } from '../hooks/useTournamentChanges';
import type { TournamentCourtFee, TournamentOwnerMatch, TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.courtFee;

/** Charging and undoing are the same kind of action on the same row: one look for both. Labels never break. */
const FEE_BUTTON = 'rounded-lg whitespace-nowrap';

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

/** A pair that paid: the amount, the method it paid with, and the undo while the match can still change. */
function PaidFee({
  name,
  side,
  target,
  actions,
  editable,
}: {
  name: string;
  side: TournamentCourtFee;
  target: CourtFeeTarget;
  actions: CourtFeeActions;
  editable: boolean;
}) {
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
      <span className="text-text-primary font-medium whitespace-nowrap">{t.paid(formatPrice(side.amount))}</span>
      {side.method && (
        <span className="text-text-tertiary min-w-0 truncate text-xs" title={methodLabel(side.method)}>
          {methodLabel(side.method)}
        </span>
      )}
      {editable && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className={FEE_BUTTON}
          aria-label={t.undoAria(name)}
          onClick={() => {
            actions.onUndo(target);
          }}
        >
          {t.undo}
        </Button>
      )}
    </div>
  );
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
    <li className="flex flex-col gap-1 text-sm sm:grid sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] sm:items-center sm:gap-x-3">
      <span className="text-text-secondary min-w-0 truncate" title={name}>
        {name}
      </span>
      {side.charged ? (
        <PaidFee name={name} side={side} target={target} actions={actions} editable={editable} />
      ) : (
        editable && (
          <div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className={FEE_BUTTON}
              aria-label={t.chargeAria(name)}
              onClick={() => {
                actions.onCharge(target);
              }}
            >
              {t.charge}
            </Button>
          </div>
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
      {fee > 0 && (
        <p className="text-text-secondary truncate text-xs" title={t.line(formatPrice(fee))}>
          {t.line(formatPrice(fee))}
        </p>
      )}
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
