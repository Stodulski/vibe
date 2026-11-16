import { TicketCourt } from '../ticket/TicketParts';
import { BOOKING_MESSAGES } from '../../messages';

export interface TicketCourtRow {
  id: string;
  /** Already kept together with `keepTogether`. */
  name: string;
  /** What a player compares, as chips: the roof, the surface, the lighting. */
  tags: string[];
}

/**
 * The courts of an entry on the green ticket, the way a booking shows its court: one
 * line of text, then the facts as chips, each fact once, so the ticket stays as short as
 * a booking's. One or two courts are named; more are counted ("6 canchas"). Nothing when
 * there are none.
 */
/** Beyond this many courts the ticket counts them instead of naming each one. */
const MAX_NAMED_COURTS = 2;

export function TicketCourtList({ courts, label }: { courts: TicketCourtRow[]; label?: string | undefined }) {
  if (courts.length === 0) return null;
  const names =
    courts.length <= MAX_NAMED_COURTS
      ? courts.map((court) => court.name).join(', ')
      : BOOKING_MESSAGES.events.courtsCount(courts.length);
  const tags = [...new Set(courts.flatMap((court) => court.tags))];
  return (
    <div>
      {label && courts.length <= MAX_NAMED_COURTS && <p className="text-text-inverse text-sm">{label}</p>}
      <div className="mt-1">
        <TicketCourt name={names} tags={tags} />
      </div>
    </div>
  );
}
