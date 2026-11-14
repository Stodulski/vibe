import { Button, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicTournamentPath } from '../../lib/publicPaths';
import type { PublicTournamentSummary } from '../../api/types';
import { formatLabel, isOpenForSignUp, sportLabel, tournamentDates } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

/** One tournament on the complex's page: its dates, its categories with price and spots, and the link to its page. */
export function TournamentCard({ tournament, slug }: { tournament: PublicTournamentSummary; slug: string }) {
  return (
    <article className="border-border-subtle bg-bg-subtle flex w-full flex-col gap-3 rounded-2xl border p-4 text-left sm:p-5">
      <div className="flex flex-col gap-1">
        <h3 className="text-text-primary text-lg font-bold">{tournament.name}</h3>
        <p className="text-text-secondary text-sm first-letter:uppercase">{tournamentDates(tournament)}</p>
        <p className="text-text-secondary text-sm">{sportLabel(tournament.sport)}</p>
      </div>

      {tournament.categories.length > 0 && (
        <ul aria-label={t.categoriesLabel} className="flex flex-col gap-1.5">
          {tournament.categories.map((category) => (
            <li key={category.id} className="flex flex-wrap items-baseline justify-between gap-x-3 text-sm">
              <span className="text-text-primary font-medium">
                {category.name} <span className="text-text-tertiary font-normal">{formatLabel(category.format)}</span>
              </span>
              <span className="text-text-secondary tabular-nums">
                {isOpenForSignUp(category)
                  ? `${formatPrice(category.total_price ?? category.price)} ${t.perPair}`
                  : category.status === 'open'
                    ? t.full
                    : t.signUpClosedCategory}
              </span>
            </li>
          ))}
        </ul>
      )}

      <Button asChild size="lg" className="min-h-12 w-full">
        <a href={publicTournamentPath(slug, tournament.id)}>
          {t.viewTournament}
          <span className="sr-only">{`: ${tournament.name}`}</span>
        </a>
      </Button>
    </article>
  );
}
