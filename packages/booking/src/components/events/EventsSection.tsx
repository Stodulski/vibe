import { BOOKING_MESSAGES } from '../../messages';
import type { PublicOpenPlayEvent } from '../../api/types';
import { EventCard } from './EventCard';

const t = BOOKING_MESSAGES.events;

/**
 * The open-play events the complex's page lists once one is picked, each with
 * the link to its own sign-up page.
 */
export function EventsSection({ events, slug }: { events: PublicOpenPlayEvent[]; slug: string }) {
  return (
    <section aria-labelledby="open-play-events-title" className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h3 id="open-play-events-title" className="text-text-primary text-xl font-bold">
          {t.sectionTitle}
        </h3>
        <p className="text-text-secondary text-sm">{t.sectionDescription}</p>
      </div>
      <ul className={`grid grid-cols-1 gap-3 ${events.length > 1 ? 'md:grid-cols-2' : ''}`}>
        {events.map((event) => (
          <li key={event.id} className="flex">
            <EventCard event={event} slug={slug} />
          </li>
        ))}
      </ul>
    </section>
  );
}
