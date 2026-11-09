import { useId } from 'react';
import { BOOKING_MESSAGES } from '../../messages';
import type { Court } from '../../api/types';

const t = BOOKING_MESSAGES;

/** What the directory shows of a court. Nothing priced: the page cannot take a booking yet. */
type DirectoryCourt = Pick<Court, 'id' | 'name' | 'sport' | 'court_type' | 'description'>;

/**
 * The complex's courts, listed for a visitor who cannot book them online.
 *
 * Read-only on purpose: there are no prices and no control, because nothing
 * here can be acted on until the club takes online bookings. The owner's
 * description is free text and is rendered as text, never as markup.
 */
export function CourtDirectory({ courts }: { courts: DirectoryCourt[] }) {
  const titleId = useId();
  if (courts.length === 0) return null;

  return (
    <section aria-labelledby={titleId} className="space-y-4 text-left">
      <h2 id={titleId} className="text-text-primary text-lg font-semibold">
        {t.publicBooking.courtsTitle}
      </h2>
      <ul className="grid gap-3 sm:grid-cols-2">
        {courts.map((court) => (
          <li
            key={court.id}
            className="border-border-subtle bg-bg-subtle rounded-2xl border p-4"
          >
              <p className="text-text-primary truncate font-semibold" title={court.name}>
                {court.name}
              </p>
              <div className="mt-1.5 flex flex-wrap gap-1.5">
                <span className="bg-primary-500/15 text-primary-300 rounded-full px-2.5 py-0.5 text-xs font-medium">
                  {t.courts.sportTypes[court.sport]}
                </span>
                <span className="border-border-subtle text-text-secondary rounded-full border px-2.5 py-0.5 text-xs">
                  {t.courts.courtTypes[court.court_type]}
                </span>
              </div>
              <p
                className={
                  court.description?.trim()
                    ? 'text-text-secondary mt-2 text-sm'
                    : 'text-text-tertiary mt-2 text-sm italic'
                }
              >
                {court.description?.trim() ? court.description : t.publicBooking.courtNoDescription}
              </p>
          </li>
        ))}
      </ul>
    </section>
  );
}

