import { BOOKING_MESSAGES } from '../../messages';
import { useComplexTournaments } from '../../hooks/useComplexTournaments';
import { TournamentCard } from './TournamentCard';

const t = BOOKING_MESSAGES.tournaments;

/**
 * The upcoming tournaments on the complex's page, each with the link to its
 * own page. Nothing renders while loading, on a failure, or when there are
 * none, so a complex without tournaments shows no section.
 */
export function TournamentsSection({ slug }: { slug: string }) {
  const { data: tournaments } = useComplexTournaments(slug);
  if (!tournaments || tournaments.length === 0) return null;

  return (
    <section aria-labelledby="tournaments-title" className="mt-8 flex flex-col gap-4 sm:mt-12">
      <div className="flex flex-col gap-1">
        <h2 id="tournaments-title" className="text-text-primary text-xl font-bold">
          {t.sectionTitle}
        </h2>
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
