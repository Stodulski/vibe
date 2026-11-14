import { BOOKING_MESSAGES } from '../../messages';
import { useComplexEvents } from '../../hooks/useComplexEvents';
import { EventCard } from './EventCard';

const t = BOOKING_MESSAGES.events;

/**
 * The upcoming open-play events on the complex's page, each with the link to
 * its own sign-up page. Nothing renders while loading, on a failure, or when
 * there are no upcoming events, so a complex without events shows no section.
 */
export function EventsSection({ slug }: { slug: string }) {
  const { data: events } = useComplexEvents(slug);
  if (!events || events.length === 0) return null;

  return (
    <section aria-labelledby="open-play-events-title" className="mt-6 flex flex-col gap-4 sm:mt-10">
      <div className="flex flex-col gap-1">
        <h2 id="open-play-events-title" className="text-text-primary text-xl font-bold">
          {t.sectionTitle}
        </h2>
        <p className="text-text-secondary text-sm">{t.sectionDescription}</p>
      </div>
      <ul className="grid grid-cols-1 gap-3 md:grid-cols-2">
        {events.map((event) => (
          <li key={event.id} className="flex">
            <EventCard event={event} slug={slug} />
          </li>
        ))}
      </ul>
    </section>
  );
}
