import { BOOKING_MESSAGES } from '../../messages';
import type { PublicTournamentSummary } from '../../api/types';
import { TournamentCard } from './TournamentCard';

const t = BOOKING_MESSAGES.tournaments;

/**
 * The tournaments the complex's page lists once one is picked, each with the
 * link to its own page. The caller passes only those worth listing: see
 * `listedTournaments`.
 */
export function TournamentsSection({ tournaments, slug }: { tournaments: PublicTournamentSummary[]; slug: string }) {
  return (
    <section aria-labelledby="tournaments-title" className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h3 id="tournaments-title" className="text-text-primary text-xl font-bold">
          {t.sectionTitle}
        </h3>
        <p className="text-text-secondary text-sm">{t.sectionDescription}</p>
      </div>
      <ul className={`grid grid-cols-1 gap-3 ${tournaments.length > 1 ? 'md:grid-cols-2' : ''}`}>
        {tournaments.map((tournament) => (
          <li key={tournament.id} className="flex">
            <TournamentCard tournament={tournament} slug={slug} />
          </li>
        ))}
      </ul>
    </section>
  );
}
