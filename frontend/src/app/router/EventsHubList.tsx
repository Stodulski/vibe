import { AlertTriangle, CalendarX2 } from 'lucide-react';
import { EventCard, EventListSkeleton } from '@/features/events';
import { TournamentCard } from '@/features/tournaments';
import { AGENDA_LIST_CLASS } from '@/shared/components/common/agendaRowLayout';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { AgendaItem, AgendaTab } from './eventsHubAgenda';

const t = ES_AR;

interface EventsHubListProps {
  items: AgendaItem[];
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  onSelect: (eventId: string) => void;
  /** Which tab this is: an empty upcoming list invites creating one, an empty past list does not. */
  kind: AgendaTab;
  onCreate: () => void;
  /** Rows are loaded, but the filters above hide every one of them. */
  hiddenByFilters: boolean;
}

/**
 * Cancha abierta and torneo rows in one column, in the order the agenda gives them. The
 * empty upcoming list is the one empty state of the page; its action is outlined because
 * the header already carries the primary Crear evento button.
 */
export function EventsHubList({
  items,
  isLoading,
  isError,
  onRetry,
  onSelect,
  kind,
  onCreate,
  hiddenByFilters,
}: EventsHubListProps) {
  if (isLoading) return <EventListSkeleton />;

  if (isError) {
    return (
      <EmptyState
        icon={AlertTriangle}
        title={t.events.loadError}
        description={t.common.loadErrorDescription}
        actionLabel={t.layout.retry}
        onAction={onRetry}
      />
    );
  }

  if (items.length === 0) {
    if (hiddenByFilters) {
      return <EmptyState icon={CalendarX2} title={t.events.noMatches} description={t.events.noMatchesDescription} />;
    }
    return kind === 'upcoming' ? (
      <EmptyState
        icon={CalendarX2}
        title={t.events.noUpcoming}
        description={t.events.noUpcomingDescription}
        actionLabel={t.events.create}
        onAction={onCreate}
        actionVariant="outline"
      />
    ) : (
      <EmptyState icon={CalendarX2} title={t.events.noPast} description="" />
    );
  }

  return (
    <ul className={AGENDA_LIST_CLASS} aria-label={t.events.listLabel}>
      {items.map((item) => (
        <li key={item.key}>
          {item.kind === 'openPlay' ? (
            <EventCard event={item.event} onSelect={onSelect} />
          ) : (
            <TournamentCard tournament={item.tournament} />
          )}
        </li>
      ))}
    </ul>
  );
}
