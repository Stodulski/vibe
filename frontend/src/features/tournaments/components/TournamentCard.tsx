import { Link } from 'react-router-dom';
import { AGENDA_ROW_CLASS } from '@/shared/components/common/agendaRowLayout';
import { AgendaRow, type AgendaRowToken } from '@/shared/components/common/AgendaRow';
import { ES_AR } from '@/shared/i18n/es_AR';
import { categoryTitle, tournamentTitle } from '@/shared/lib/traitLabels';
import { tournamentDays } from '../lib/tournamentView';
import type { TournamentOwnerSummary } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

/** A row of the tournaments list: the day it starts, its sport as the kind, the categories as the title, then its days, categories and pairs. Pressing it opens the detail. */
export function TournamentCard({ tournament }: { tournament: TournamentOwnerSummary }) {
  const live = tournament.categories.filter((category) => category.status !== 'cancelled');
  const pairs = live.reduce((total, category) => total + category.confirmed_count, 0);
  const categoryTitles = live.map((category) => categoryTitle(category));
  const meta: AgendaRowToken[] = [
    { key: 'days', text: tournamentDays(tournament.start_date, tournament.end_date), capitalized: true },
    { key: 'categories', text: t.categoriesCount(live.length) },
    { key: 'pairs', text: t.pairsCount(pairs) },
  ];

  return (
    <Link to={`/events/tournaments/${tournament.id}`} className={AGENDA_ROW_CLASS}>
      <AgendaRow
        date={tournament.start_date}
        kicker={tournamentTitle(tournament.sport)}
        title={categoryTitles.length > 0 ? categoryTitles.join(', ') : t.categoriesCount(0)}
        badge={
          tournament.status === 'cancelled' ? (
            <span className="text-error-text shrink-0 text-xs font-medium">{t.cancelledBadge}</span>
          ) : null
        }
        meta={meta}
      />
    </Link>
  );
}
