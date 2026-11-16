export interface TicketCourtRow {
  id: string;
  /** Already kept together with `keepTogether`. */
  name: string;
  /** What a player compares, as the booking flow words it ("Techada · Césped sintético · Con iluminación"). */
  facts: string;
}

/** The courts of an entry on the green ticket: one per row, the name and then its facts. Nothing when there are none. */
export function TicketCourtList({ courts, label }: { courts: TicketCourtRow[]; label?: string | undefined }) {
  if (courts.length === 0) return null;
  return (
    <div>
      {label && <p className="text-text-inverse text-sm">{label}</p>}
      <ul className="mt-1 space-y-2">
        {courts.map((court) => (
          <li key={court.id}>
            <p className="text-text-inverse text-base font-semibold">{court.name}</p>
            <p className="text-text-inverse text-sm">{court.facts}</p>
          </li>
        ))}
      </ul>
    </div>
  );
}
