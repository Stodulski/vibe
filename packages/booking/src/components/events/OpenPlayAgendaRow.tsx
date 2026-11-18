import { formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicEventPath } from '../../lib/publicPaths';
import { AgendaRowShell, type AgendaMetaToken } from '../agenda/AgendaRowShell';
import { spotsNotice } from '../agenda/agendaModel';
import { eventTimeRange, spotsLeftLabel } from './eventFormat';
import type { OpenPlayRow } from './openPlayAgendaModel';

const t = BOOKING_MESSAGES.events;
const w = BOOKING_MESSAGES.waitlist;

interface OpenPlayAgendaRowProps {
  row: OpenPlayRow;
  slug: string;
  onViewDetail: (row: OpenPlayRow) => void;
  /** Opens the waitlist of a full event. */
  onNotify: (row: OpenPlayRow) => void;
}

/** One open-play event on the agenda: its time, its places and its price on one line, and its sign-up, or "Avisarme" when full. */
export function OpenPlayAgendaRow({ row, slug, onViewDetail, onNotify }: OpenPlayAgendaRowProps) {
  const { event } = row;
  const full = event.spots_left <= 0;
  const spots = full
    ? { text: w.full, last: true }
    : spotsNotice(event.spots_left, { lastSpots: t.lastSpots, spotsLeft: spotsLeftLabel });
  const meta: AgendaMetaToken[] = [
    { key: 'spots', text: spots.text, emphasis: spots.last },
    { key: 'price', text: formatPrice(event.price), unit: t.perPersonShort },
  ];

  return (
    <AgendaRowShell
      date={row.date}
      label={row.label}
      title={row.title}
      time={eventTimeRange(event)}
      meta={meta}
      viewDetailLabel={t.viewDetail}
      onViewDetail={() => {
        onViewDetail(row);
      }}
      signUp={
        full
          ? {
              notifyLabel: w.notifyMe,
              onNotify: () => {
                onNotify(row);
              },
            }
          : { href: publicEventPath(slug, event.id), label: t.signUp }
      }
    />
  );
}
