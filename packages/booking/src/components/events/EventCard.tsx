import { Button } from '@vibe/ui';
import { formatDateFull, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { keepTogether } from '../../lib/keepTogether';
import { publicEventPath } from '../../lib/publicPaths';
import { openPlayTitle } from '../../lib/traitLabels';
import type { PublicOpenPlayEvent } from '../../api/types';
import { eventHasStarted, eventTimeRange, isOpenForSignUp, spotsLeftLabel, sportLabel } from './eventFormat';

const t = BOOKING_MESSAGES.events;

interface EventCardProps {
  event: PublicOpenPlayEvent;
  /** Set on the complex page: the card links to the event's own page. Unset on that page itself. */
  slug?: string;
  headingLevel?: 'h1' | 'h2' | 'h3';
  /** Centavos ARS the person paid, on their own registration: shown in place of the price and the spots left. */
  paid?: number;
}

/** Why a card offers no sign-up: cancelled beats started, which beats full. */
function closedLabel(event: PublicOpenPlayEvent): string {
  if (event.status === 'cancelled') return t.cancelledBadge;
  if (eventHasStarted(event)) return t.closedBadge;
  return t.full;
}

/** One open-play event: what it is, when, where, what it costs and how many spots are left. */
export function EventCard({ event, slug, headingLevel = 'h3', paid }: EventCardProps) {
  const Heading = headingLevel;
  // A full, cancelled or started event says so once, in the button or in the page state below the card.
  const open = isOpenForSignUp(event) && !eventHasStarted(event);

  return (
    <article className="border-border-subtle flex w-full flex-col gap-3 rounded-2xl border p-4 text-left sm:p-5">
      <div className="flex flex-col gap-1">
        <Heading className="text-text-primary text-xl leading-tight font-bold">{openPlayTitle(event)}</Heading>
        <p className="text-text-secondary text-sm first-letter:uppercase">
          {formatDateFull(`${event.date}T12:00:00`)}
        </p>
        <p className="text-text-primary text-base font-bold whitespace-nowrap tabular-nums">{eventTimeRange(event)}</p>
        <p className="text-text-secondary text-sm whitespace-nowrap">{sportLabel(event.sport)}</p>
      </div>

      {event.court_names.length > 0 && (
        <p className="text-text-secondary text-sm">
          <span className="text-text-tertiary">{t.courtsLabel}: </span>
          {event.court_names.map(keepTogether).join(', ')}
        </p>
      )}

      {paid !== undefined ? (
        <p className="text-text-primary text-base font-semibold whitespace-nowrap tabular-nums">
          {`${t.paidPrefix} ${formatPrice(paid)}`}
        </p>
      ) : (
        <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
          <p className="text-text-primary text-base font-semibold whitespace-nowrap tabular-nums">
            {formatPrice(event.price)} <span className="text-text-secondary text-sm font-normal">{t.perPerson}</span>
          </p>
          {open && <p className="text-primary-400 text-sm font-medium">{spotsLeftLabel(event.spots_left)}</p>}
        </div>
      )}

      {slug !== undefined &&
        (open ? (
          <Button asChild size="lg" className="min-h-12 w-full sm:w-auto sm:self-start">
            <a href={publicEventPath(slug, event.id)}>{t.signUp}</a>
          </Button>
        ) : (
          <p className="text-text-secondary text-sm font-medium">{closedLabel(event)}</p>
        ))}
    </article>
  );
}
