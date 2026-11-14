import { Link } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { tournamentDays } from '../lib/tournamentView';
import { TOURNAMENT_CARD_CLASS, TOURNAMENT_CARD_TOP_ROW_CLASS } from '../lib/tournamentLayout';
import type { TournamentOwnerSummary } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

/** A row of the tournaments list: name, days, categories and how many pairs signed up. Pressing it opens the detail. */
export function TournamentCard({ tournament }: { tournament: TournamentOwnerSummary }) {
  const live = tournament.categories.filter((category) => category.status !== 'cancelled');
  const pairs = live.reduce((total, category) => total + category.confirmed_count, 0);

  return (
    <Link
      to={`/events/tournaments/${tournament.id}`}
      className={cn(TOURNAMENT_CARD_CLASS, 'focus-self hover:border-border-default transition-colors')}
    >
      <div className={TOURNAMENT_CARD_TOP_ROW_CLASS}>
        <p className="text-text-primary min-w-0 truncate text-base font-semibold">{tournament.name}</p>
        {tournament.status === 'cancelled' && (
          <span className="text-error-text shrink-0 text-xs font-medium">{t.cancelledBadge}</span>
        )}
      </div>
      <p className="text-text-secondary text-sm first-letter:uppercase">
        {tournamentDays(tournament.start_date, tournament.end_date)}
      </p>
      <p className="text-text-tertiary truncate text-xs" title={live.map((c) => c.name).join(', ')}>
        {t.categoriesCount(live.length)} · {live.map((c) => c.name).join(', ')}
      </p>
      <p className="text-text-secondary mt-1.5 text-sm">{t.pairsCount(pairs)}</p>
    </Link>
  );
}
