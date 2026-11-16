import { AlertTriangle, Trophy } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { ES_AR } from '@/shared/i18n/es_AR';
import { TOURNAMENT_LIST_CLASS } from '../lib/tournamentLayout';
import { TournamentCard } from './TournamentCard';
import { TournamentListSkeleton } from './TournamentListSkeleton';
import type { TournamentOwnerSummary } from '@/shared/types/api.types';

const t = ES_AR;

interface TournamentListProps {
  tournaments: TournamentOwnerSummary[];
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  /** An empty upcoming list invites creating one, an empty past list does not. */
  kind: 'upcoming' | 'past';
}

/** The empty list points at the header's "Crear torneo": one primary button per screen, not a second one here. */
export function TournamentList({ tournaments, isLoading, isError, onRetry, kind }: TournamentListProps) {
  if (isLoading) return <TournamentListSkeleton />;

  if (isError) {
    return (
      <EmptyState
        icon={AlertTriangle}
        title={t.tournaments.loadError}
        description={t.common.loadErrorDescription}
        actionLabel={t.layout.retry}
        onAction={onRetry}
      />
    );
  }

  if (tournaments.length === 0) {
    return kind === 'upcoming' ? (
      <EmptyState icon={Trophy} title={t.tournaments.noUpcoming} description={t.tournaments.noUpcomingDescription} />
    ) : (
      <EmptyState icon={Trophy} title={t.tournaments.noPast} description="" />
    );
  }

  return (
    <ul className={TOURNAMENT_LIST_CLASS} aria-label={t.tournaments.listLabel}>
      {tournaments.map((tournament) => (
        <li key={tournament.id} className="flex">
          <TournamentCard tournament={tournament} />
        </li>
      ))}
    </ul>
  );
}
