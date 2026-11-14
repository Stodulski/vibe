import { AlertTriangle, CalendarX2 } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { ES_AR } from '@/shared/i18n/es_AR';
import { EVENT_LIST_CLASS } from '../lib/eventLayout';
import { EventCard } from './EventCard';
import { EventListSkeleton } from './EventListSkeleton';
import type { OpenPlayEvent } from '@/shared/types/api.types';

const t = ES_AR;

interface EventListProps {
  events: OpenPlayEvent[];
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  onSelect: (eventId: string) => void;
  /** Which tab this is: an empty upcoming list invites creating one, an empty past list does not. */
  kind: 'upcoming' | 'past';
  onCreate: () => void;
}

export function EventList({ events, isLoading, isError, onRetry, onSelect, kind, onCreate }: EventListProps) {
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

  if (events.length === 0) {
    return kind === 'upcoming' ? (
      <EmptyState
        icon={CalendarX2}
        title={t.events.noUpcoming}
        description={t.events.noUpcomingDescription}
        actionLabel={t.events.create}
        onAction={onCreate}
      />
    ) : (
      <EmptyState icon={CalendarX2} title={t.events.noPast} description="" />
    );
  }

  return (
    <ul className={EVENT_LIST_CLASS} aria-label={t.events.listLabel}>
      {events.map((event) => (
        <li key={event.id} className="flex">
          <EventCard event={event} onSelect={onSelect} />
        </li>
      ))}
    </ul>
  );
}
