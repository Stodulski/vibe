import { AGENDA_ROW_CLASS } from '@/shared/components/common/agendaRowLayout';
import { AgendaRow, type AgendaRowToken } from '@/shared/components/common/AgendaRow';
import { keepTogether } from '@/shared/lib/keepTogether';
import { ES_AR } from '@/shared/i18n/es_AR';
import { openPlayAudience, openPlayKind } from '@/shared/lib/traitLabels';
import { formatPrice } from '@/shared/lib/utils';
import { formatEventTimeRange } from '@/shared/lib/eventDates';
import type { OpenPlayEvent } from '@/shared/types/api.types';

const t = ES_AR.events;

/** One word of state beside the name, by colour alone like the booking statuses; nothing for a plain open event. */
function EventStateText({ event }: { event: OpenPlayEvent }) {
  if (event.status === 'cancelled') {
    return <span className="text-error-text shrink-0 text-xs font-medium">{t.cancelledBadge}</span>;
  }
  if (event.spots_left === 0) {
    return <span className="text-success-text shrink-0 text-xs font-medium">{t.fullBadge}</span>;
  }
  return null;
}

/** The courts as the row names them: their names when there are two or fewer, otherwise how many. */
function courtsToken(event: OpenPlayEvent): string {
  if (event.courts.length <= 2) return event.courts.map((court) => keepTogether(court.name)).join(', ');
  return t.courtsCount(event.courts.length);
}

/** The meta line of an open play: its hours, the places taken, the price and the courts. */
function eventMeta(event: OpenPlayEvent): AgendaRowToken[] {
  const meta: AgendaRowToken[] = [
    { key: 'hours', text: formatEventTimeRange(event.start_time, event.end_time) },
    { key: 'spots', text: t.spotsTaken(event.confirmed_count, event.capacity) },
    { key: 'price', text: formatPrice(event.price) },
  ];
  if (event.courts.length > 0) meta.push({ key: 'courts', text: courtsToken(event) });
  return meta;
}

/** A row of the events list: its day, what it is, who it is for, when, how full, the price and the courts. Pressing it opens the detail. */
export function EventCard({ event, onSelect }: { event: OpenPlayEvent; onSelect: (eventId: string) => void }) {
  return (
    <button
      type="button"
      onClick={() => {
        onSelect(event.id);
      }}
      className={AGENDA_ROW_CLASS}
    >
      <AgendaRow
        date={event.date}
        kicker={openPlayKind(event)}
        title={openPlayAudience(event)}
        badge={<EventStateText event={event} />}
        meta={eventMeta(event)}
      />
    </button>
  );
}
