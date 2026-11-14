import { Button } from '@vibe/ui';
import { formatDateFull, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicEventPath } from '../../lib/publicPaths';
import type { PublicOpenPlayEvent } from '../../api/types';
import { eventTimeRange, isOpenForSignUp, spotsLeftLabel, sportLabel } from './eventFormat';

const t = BOOKING_MESSAGES.events;

interface EventCardProps {
  event: PublicOpenPlayEvent;
  /** Set on the complex page: the card links to the event's own page. Unset on that page itself. */
  slug?: string;
  headingLevel?: 'h1' | 'h2' | 'h3';
}

/** One open-play event: what it is, when, where, what it costs and how many spots are left. */
export function EventCard({ event, slug, headingLevel = 'h3' }: EventCardProps) {
  const Heading = headingLevel;
  const open = isOpenForSignUp(event);
  const cancelled = event.status === 'cancelled';

  return (
    <article className="border-border-subtle bg-bg-subtle flex w-full flex-col gap-3 rounded-2xl border p-4 text-left sm:p-5">
      <div className="flex flex-col gap-1">
        <Heading className="text-text-primary text-lg font-bold">{event.name}</Heading>
        <p className="text-text-secondary text-sm first-letter:uppercase">
          {formatDateFull(`${event.date}T12:00:00`)}
        </p>
        <p className="text-text-primary text-2xl font-bold tabular-nums">{eventTimeRange(event)}</p>
        <p className="text-text-secondary text-sm">{sportLabel(event.sport)}</p>
      </div>

      {event.court_names.length > 0 && (
        <p className="text-text-secondary text-sm">
          <span className="text-text-tertiary">{t.courtsLabel}: </span>
          {event.court_names.join(', ')}
        </p>
      )}

      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <p className="text-text-primary text-base font-semibold tabular-nums">
          {formatPrice(event.total_price)} <span className="text-text-secondary text-sm font-normal">{t.perPerson}</span>
        </p>
        <p className={open ? 'text-primary-400 text-sm font-medium' : 'text-text-secondary text-sm font-medium'}>
          {cancelled ? BOOKING_MESSAGES.events.cancelledTitle : open ? spotsLeftLabel(event.spots_left) : t.full}
        </p>
      </div>

      {slug !== undefined && (
        <Button asChild={open} disabled={!open} size="lg" className="min-h-12 w-full">
          {open ? <a href={publicEventPath(slug, event.id)}>{t.signUp}</a> : <span>{t.full}</span>}
        </Button>
      )}
    </article>
  );
}
