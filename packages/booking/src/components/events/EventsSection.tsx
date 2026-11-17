import { BOOKING_MESSAGES } from '../../messages';
import { EventsAgenda } from './EventsAgenda';
import type { EventRow } from './eventRows';

const t = BOOKING_MESSAGES.events;

/**
 * The complex's events under one heading, "Eventos": its open play and its
 * tournaments in one list, soonest first, with the note that it lists only
 * what has a place. The caller passes the rows: see `eventRows`.
 */
export function EventsSection({ rows, slug }: { rows: EventRow[]; slug: string }) {
  return (
    <section aria-labelledby="events-title" className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h3 id="events-title" className="text-text-primary text-xl font-bold">
          {t.pickerTitle}
        </h3>
        <p className="text-text-secondary text-sm">{t.onlyWithSpots}</p>
      </div>
      <EventsAgenda rows={rows} slug={slug} />
    </section>
  );
}
