import { TicketCourt } from '../ticket/TicketParts';
import { courtNamesLine, MAX_NAMED_COURTS } from '../../lib/courtNames';

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
export function TicketCourtList({
  courts,
  label,
  bare = false,
}: {
  courts: TicketCourtRow[];
  label?: string | undefined;
  bare?: boolean;
}) {
  if (courts.length === 0) return null;
  const names = courtNamesLine(courts.map((court) => court.name));
  const tags = [...new Set(courts.flatMap((court) => court.tags))];
  return (
    <div>
      {label && courts.length <= MAX_NAMED_COURTS && <p className="text-text-inverse text-sm">{label}</p>}
      <div className="mt-1">
        <TicketCourt name={names} tags={tags} bare={bare} />
      </div>
    </div>
  );
}
