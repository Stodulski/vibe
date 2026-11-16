import { Button, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicTournamentPath } from '../../lib/publicPaths';
import type { PublicTournamentSummary } from '../../api/types';
import { formatLabel, isOpenForSignUp, isSignUpClosed, sportLabel, tournamentDates } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

/** What a category row says at its end: the price while it takes pairs, otherwise why it does not. */
function categoryStatus(category: PublicTournamentSummary['categories'][number]): string {
  if (isOpenForSignUp(category)) return `${formatPrice(category.price)} ${t.perPair}`;
  if (category.status === 'cancelled') return t.categoryStatusCancelled;
  if (isSignUpClosed(category)) return t.categoryStatusClosed;
  return t.full;
}

/**
 * One tournament on the complex's page: its dates, its categories with price and spots, and the link to its page.
 * Each category takes two lines at every width, its name and price, then its format, so no row squeezes into columns.
 */
export function TournamentCard({ tournament, slug }: { tournament: PublicTournamentSummary; slug: string }) {
  return (
    <article className="border-border-subtle flex w-full flex-col gap-3 rounded-2xl border p-4 text-left sm:p-5">
      <div className="flex flex-col gap-1">
        <h3 className="text-text-primary text-xl leading-tight font-bold">{tournament.name}</h3>
        <p className="text-text-secondary text-sm whitespace-nowrap">{tournamentDates(tournament)}</p>
        <p className="text-text-secondary text-sm whitespace-nowrap">{sportLabel(tournament.sport)}</p>
      </div>

      {tournament.categories.length > 0 && (
        <ul aria-label={t.categoriesLabel} className="divide-border-subtle flex flex-col divide-y">
          {tournament.categories.map((category) => (
            <li key={category.id} className="flex flex-col gap-0.5 py-2 text-sm first:pt-0 last:pb-0">
              <div className="flex min-w-0 items-baseline justify-between gap-3">
                <span className="text-text-primary min-w-0 truncate font-medium">{category.name}</span>
                <span className="text-text-secondary shrink-0 whitespace-nowrap tabular-nums">
                  {categoryStatus(category)}
                </span>
              </div>
              <span className="text-text-tertiary text-xs">{formatLabel(category.format)}</span>
            </li>
          ))}
        </ul>
      )}

      <Button asChild size="lg" className="min-h-12 w-full sm:w-auto sm:self-start">
        <a href={publicTournamentPath(slug, tournament.id)}>
          {t.viewTournament}
          <span className="sr-only">{`: ${tournament.name}`}</span>
        </a>
      </Button>
    </article>
  );
}
