import { useState } from 'react';
import { BOOKING_MESSAGES } from '../../messages';
import { AgendaDetailDialog } from '../agenda/AgendaDetailDialog';
import { AgendaFilters } from '../agenda/AgendaFilters';
import { type AgendaFilter } from '../agenda/agendaModel';
import { useLoadMoreOnSentinel } from '../agenda/useLoadMoreOnSentinel';
import { AgendaRow } from '../tournaments/AgendaRow';
import { OpenPlayAgendaRow } from './OpenPlayAgendaRow';
import {
  eventDetailOf,
  eventModalityOptions,
  eventRowKey,
  filterEventRows,
  type EventRow,
} from './eventRows';

const t = BOOKING_MESSAGES.events;

/** The rows the list shows at first, and how many more each time its end comes into view. */
export const PAGE_SIZE = 10;

interface EventsListProps {
  items: EventRow[];
  slug: string;
  onViewDetail: (item: EventRow) => void;
}

/** One page of rows, each drawn as its kind draws it. */
function EventsList({ items, slug, onViewDetail }: EventsListProps) {
  return (
    <ul className="divide-border-subtle flex flex-col divide-y">
      {items.map((item) => (
        <li key={eventRowKey(item)}>
          {item.kind === 'open-play' ? (
            <OpenPlayAgendaRow
              row={item.row}
              slug={slug}
              onViewDetail={(row) => {
                onViewDetail({ kind: 'open-play', row });
              }}
            />
          ) : (
            <AgendaRow
              row={item.row}
              slug={slug}
              onViewDetail={(row) => {
                onViewDetail({ kind: 'tournament', row });
              }}
            />
          )}
        </li>
      ))}
    </ul>
  );
}

interface EventsAgendaProps {
  /** Every row of the complex's events, of either kind, soonest first (see `eventRows`). */
  rows: EventRow[];
  slug: string;
}

/**
 * The Eventos list: the filters for modality and day above one list of both
 * kinds. The list shows a page of rows, and the next page when the end of it
 * scrolls into view. A filter change shows the first page again. Each row's
 * detail opens in one dialog.
 */
export function EventsAgenda({ rows, slug }: EventsAgendaProps) {
  const [filter, setFilter] = useState<AgendaFilter>({ modality: 'all', day: null });
  const [shown, setShown] = useState(PAGE_SIZE);
  const [detail, setDetail] = useState<EventRow | null>(null);
  const visible = filterEventRows(rows, filter);
  const page = visible.slice(0, shown);
  const hasMore = page.length < visible.length;
  const sentinelRef = useLoadMoreOnSentinel(
    () => {
      setShown((count) => count + PAGE_SIZE);
    },
    hasMore,
    shown,
  );

  return (
    <div className="flex flex-col gap-4">
      <AgendaFilters
        copy={{ modality: t.filterModality, allModalities: t.filterAll }}
        modalities={eventModalityOptions(rows)}
        filter={filter}
        onChange={(next) => {
          setFilter(next);
          setShown(PAGE_SIZE);
        }}
      />

      {visible.length === 0 ? (
        <p className="text-text-secondary text-sm">{t.noAgendaRows}</p>
      ) : (
        <EventsList items={page} slug={slug} onViewDetail={setDetail} />
      )}

      {hasMore && <div ref={sentinelRef} aria-hidden="true" />}

      <AgendaDetailDialog
        detail={detail && eventDetailOf(detail, slug)}
        closeLabel={t.closeDetail}
        onClose={() => {
          setDetail(null);
        }}
      />
    </div>
  );
}
