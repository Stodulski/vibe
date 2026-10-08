import { useId } from 'react';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Court } from '@/shared/types/api.types';

const t = ES_AR;

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
          <li key={court.id} className="border-border-subtle bg-bg-subtle rounded-xl border p-4">
            <p className="text-text-primary font-medium">{court.name}</p>
            <p className="text-text-secondary text-sm">
              {t.courts.sportTypes[court.sport]} · {t.courts.courtTypes[court.court_type]}
            </p>
            {court.description ? <p className="text-text-tertiary mt-2 text-sm">{court.description}</p> : null}
          </li>
        ))}
      </ul>
    </section>
  );
}
