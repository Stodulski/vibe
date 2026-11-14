import { Button, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicTournamentPath } from '../../lib/publicPaths';
import type { PublicTournamentSummary } from '../../api/types';
import { formatLabel, isOpenForSignUp, pairPriceSuffix, sportLabel, tournamentDates } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

/** What a category row says at its end: the price while it takes pairs, otherwise why it does not. */
function categoryStatus(category: PublicTournamentSummary['categories'][number]): string {
  if (isOpenForSignUp(category)) return `${formatPrice(category.price)} ${pairPriceSuffix(category)}`;
  if (category.status === 'cancelled') return t.categoryStatusCancelled;
  if (category.status === 'open') return t.full;
  return t.categoryStatusClosed;
}

/** One tournament on the complex's page: its dates, its categories with price and spots, and the link to its page. */
export function TournamentCard({ tournament, slug }: { tournament: PublicTournamentSummary; slug: string }) {
  return (
    <article className="border-border-subtle bg-bg-subtle flex w-full flex-col gap-3 rounded-2xl border p-4 text-left sm:p-5">
      <div className="flex flex-col gap-1">
        <h3 className="text-text-primary text-2xl leading-tight font-bold">{tournament.name}</h3>
        <p className="text-text-primary text-base font-bold first-letter:uppercase">{tournamentDates(tournament)}</p>
        <p className="text-text-secondary text-sm">{sportLabel(tournament.sport)}</p>
      </div>

      {tournament.categories.length > 0 && (
        <ul aria-label={t.categoriesLabel} className="divide-border-subtle flex flex-col divide-y">
          {tournament.categories.map((category) => (
            <li
              key={category.id}
              className="grid grid-cols-[minmax(0,1fr)_auto] items-baseline gap-x-4 py-2 text-sm first:pt-0 last:pb-0 sm:grid-cols-[minmax(0,1fr)_minmax(0,12rem)_auto]"
            >
              <span className="text-text-primary col-start-1 font-medium">{category.name}</span>
              <span className="text-text-tertiary col-start-1 row-start-2 sm:col-start-2 sm:row-start-1">
                {formatLabel(category.format)}
              </span>
              <span className="text-text-secondary col-start-2 row-span-2 row-start-1 text-right tabular-nums sm:col-start-3 sm:row-span-1">
                {categoryStatus(category)}
              </span>
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
