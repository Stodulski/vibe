import { toDisplayDate } from '@vibe/ui';
import { format } from 'date-fns/format';
import { es } from 'date-fns/locale/es';
import { courtFactParts } from '../../lib/courtFacts';
import { keepTogether } from '../../lib/keepTogether';
import { openPlayTitle } from '../../lib/traitLabels';
import { BOOKING_MESSAGES } from '../../messages';
import type { PublicOpenPlayEvent } from '../../api/types';
import { SignUpSummaryCard } from '../signup/SignUpSummaryCard';
import { eventTimeRange, endsNextDay, sportLabel } from './eventFormat';

const t = BOOKING_MESSAGES;

function eventDetails(event: PublicOpenPlayEvent): string[] {
  return endsNextDay(event) ? [t.events.nextDayNote] : [];
}

/** One row per court: its name kept together, then roof, surface and lighting as chips, the way the booking ticket shows a court. */
function eventCourts(event: PublicOpenPlayEvent) {
  return event.courts.map((court) => ({ id: court.id, name: keepTogether(court.name), tags: courtFactParts(court) }));
}

/**
 * The ticket of what the person signs up for in an open play: the complex and
 * the day, the hours, the event's title with its sport, one row per court and
 * the price rows. The checkout page sets it in its green band, above the form.
 */
export function EventSignUpSummary({ event }: { event: PublicOpenPlayEvent }) {
  return (
    <SignUpSummaryCard
      bare
      ariaLabel={t.events.summaryLabel}
      venue={event.complex_name}
      dateLabel={format(toDisplayDate(event.date), "EEEE d 'de' MMMM", { locale: es })}
      timeLabel={eventTimeRange(event)}
      title={openPlayTitle(event)}
      tags={[sportLabel(event.sport)]}
      details={eventDetails(event)}
      courts={eventCourts(event)}
      courtsLabel={t.events.courtsLabel}
      priceLabel={t.events.priceLabel}
      price={event.price}
      serviceFee={event.service_fee}
      total={event.total_price}
      feeNote={t.events.feeNote}
      cancellationHours={event.cancellation_hours}
    />
  );
}
