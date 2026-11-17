import { formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicEventPath } from '../../lib/publicPaths';
import { AgendaRowShell, type AgendaMetaToken } from '../agenda/AgendaRowShell';
import { spotsNotice } from '../agenda/agendaModel';
import { eventTimeRange, spotsLeftLabel } from './eventFormat';
import type { OpenPlayRow } from './openPlayAgendaModel';

const t = BOOKING_MESSAGES.events;

interface OpenPlayAgendaRowProps {
  row: OpenPlayRow;
  slug: string;
  onViewDetail: (row: OpenPlayRow) => void;
}

/** One open-play event on the agenda: its time, its places and its price on one line, and its sign-up. */
export function OpenPlayAgendaRow({ row, slug, onViewDetail }: OpenPlayAgendaRowProps) {
  const { event } = row;
  const spots = spotsNotice(event.spots_left, { lastSpots: t.lastSpots, spotsLeft: spotsLeftLabel });
  const meta: AgendaMetaToken[] = [
    { key: 'time', text: eventTimeRange(event) },
    { key: 'spots', text: spots.text, emphasis: spots.last },
    { key: 'price', text: `${formatPrice(event.price)} ${t.perPerson}` },
  ];

  return (
    <AgendaRowShell
      date={row.date}
      label={row.label}
      title={row.title}
      meta={meta}
      viewDetailLabel={t.viewDetail}
      onViewDetail={() => {
        onViewDetail(row);
      }}
      signUp={{ href: publicEventPath(slug, event.id), label: t.signUp }}
    />
  );
}
