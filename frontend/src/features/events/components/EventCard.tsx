import { ES_AR } from '@/shared/i18n/es_AR';
import { cn, formatDateFull, formatPrice, formatTime } from '@/shared/lib/utils';
import { EVENT_CARD_BOTTOM_ROW_CLASS, EVENT_CARD_CLASS, EVENT_CARD_TOP_ROW_CLASS } from '../lib/eventLayout';
import type { OpenPlayEvent } from '@/shared/types/api.types';

const t = ES_AR.events;

/** One word of state beside the name, by colour alone like the booking statuses; nothing for a plain open event. */
function EventStateText({ event }: { event: OpenPlayEvent }) {
  if (event.status === 'cancelled') {
    return <span className="text-error-text shrink-0 text-xs font-medium">{t.cancelledBadge}</span>;
  }
  if (event.spots_left === 0) {
    return <span className="text-warning-text shrink-0 text-xs font-medium">{t.fullBadge}</span>;
  }
  return null;
}

/** A row of the events list: what, when, where, how full and the price. Pressing it opens the detail. */
export function EventCard({ event, onSelect }: { event: OpenPlayEvent; onSelect: (eventId: string) => void }) {
  const courtNames = event.courts.map((court) => court.name).join(', ');

  return (
    <button
      type="button"
      onClick={() => {
        onSelect(event.id);
      }}
      className={cn(EVENT_CARD_CLASS, 'focus-self hover:border-border-default cursor-pointer transition-colors')}
    >
      <div className={EVENT_CARD_TOP_ROW_CLASS}>
        <p className="text-text-primary min-w-0 truncate text-base font-semibold">{event.name}</p>
        <EventStateText event={event} />
      </div>
      <p className="text-text-secondary text-sm">
        <span className="first-letter:uppercase">{formatDateFull(event.date)}</span>
        {' · '}
        <span className="score-text">
          {formatTime(event.start_time)} – {formatTime(event.end_time)}
        </span>
      </p>
      <p className="text-text-tertiary truncate text-xs" title={courtNames}>
        {t.courtsCount(event.courts.length)} · {courtNames}
      </p>
      <div className={EVENT_CARD_BOTTOM_ROW_CLASS}>
        <span className="text-text-secondary text-sm">{t.spotsTaken(event.confirmed_count, event.capacity)}</span>
        <span className="score-text text-text-primary text-sm font-medium">{formatPrice(event.price)}</span>
      </div>
    </button>
  );
}
