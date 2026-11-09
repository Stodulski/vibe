/**
 * Pieces shared by the two green tickets, the summary before paying and the
 * confirmed booking, so both read as the same ticket.
 */

/**
 * The tear line between the booking details and the money: a dashed rule
 * with two notches carved out of the card's edges, filled with the page's
 * background colour. The negative margins match the cards' padding
 * (p-6, sm:p-8) so the notches sit on the edge.
 */
export function TicketStub() {
  return (
    <div className="relative -mx-6 my-6 sm:-mx-8">
      <div className="border-text-inverse/30 border-t border-dashed" />
      <span
        aria-hidden="true"
        className="bg-bg-base absolute top-1/2 left-0 size-3 -translate-x-1/2 -translate-y-1/2 rounded-full"
      />
      <span
        aria-hidden="true"
        className="bg-bg-base absolute top-1/2 right-0 size-3 translate-x-1/2 -translate-y-1/2 rounded-full"
      />
    </div>
  );
}

interface TicketCourtProps {
  name: string;
  /** Sport and court type, shown as chips in one row. */
  tags: string[];
  /** The owner's free text, on its own line below the chips. */
  description?: string | null | undefined;
}

/**
 * The court under its name: sport and type as chips in one row, then the
 * owner's description on its own line, since it is free text of any length.
 * Whatever the data lacks is not rendered.
 */
export function TicketCourt({ name, tags, description }: TicketCourtProps) {
  return (
    <div>
      <p className="text-text-inverse text-base font-semibold">{name}</p>
      {tags.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {tags.map((tag) => (
            <span
              key={tag}
              className="bg-text-inverse/10 text-text-inverse rounded-full px-2.5 py-0.5 text-xs font-medium"
            >
              {tag}
            </span>
          ))}
        </div>
      )}
      {description && <p className="text-text-inverse mt-2 text-sm">{description}</p>}
    </div>
  );
}
