import { BOOKING_MESSAGES } from '../../messages';
import type {
  PublicTournamentCategory,
  PublicTournamentGroup,
  PublicTournamentMatch,
  PublicTournamentPair,
} from '../../api/types';
import { MatchLine } from './MatchLine';
import { groupMatchdays, pairLines, pairLookup, qualifiersNote, scheduleLabel, type PairLookup } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

function StandingsTable({
  group,
  category,
  lookup,
}: {
  group: PublicTournamentGroup;
  category: PublicTournamentCategory;
  lookup: PairLookup;
}) {
  const headers = [t.colPosition, t.colPair, t.colPlayed, t.colWon, t.colLost, t.colSets, t.colGames];
  return (
    <div className="border-border-subtle max-w-full min-w-0 overflow-x-auto rounded-xl border">
      <table aria-label={`${t.standingsLabelPrefix} ${group.name}`} className="w-full text-sm">
        <thead>
          <tr className="text-text-tertiary border-border-subtle border-b text-left text-xs">
            {headers.map((header, index) => (
              <th key={header} scope="col" className={index < 2 ? 'px-3 py-2 font-medium' : 'px-2 py-2 text-right font-medium'}>
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {group.standings.map((row) => {
            const qualifies = row.rank <= category.qualifiers_per_group;
            return (
              <tr
                key={row.pair_id}
                data-qualifies={qualifies ? 'true' : 'false'}
                className={`border-border-subtle border-b last:border-b-0 ${qualifies ? '[&>td:first-child]:border-l-primary-400 [&>td:first-child]:border-l-2' : ''}`}
              >
                <td className="text-text-secondary px-3 py-2 tabular-nums">{row.rank}</td>
                <td className="text-text-primary min-w-40 px-3 py-2">
                  {pairLines(lookup.get(row.pair_id)).map((line) => (
                    <span key={line} className="block">
                      {line}
                    </span>
                  ))}
                </td>
                <td className="px-2 py-2 text-right tabular-nums">{row.played}</td>
                <td className="px-2 py-2 text-right tabular-nums">{row.won}</td>
                <td className="px-2 py-2 text-right tabular-nums">{row.lost}</td>
                <td className="px-2 py-2 text-right whitespace-nowrap tabular-nums">{`${String(row.sets_for)}-${String(row.sets_against)}`}</td>
                <td className="px-3 py-2 text-right whitespace-nowrap tabular-nums">{`${String(row.games_for)}-${String(row.games_against)}`}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function matchCaption(match: PublicTournamentMatch): string | undefined {
  const s = match.schedule;
  return s ? scheduleLabel(s) : undefined;
}

interface GroupSectionProps {
  group: PublicTournamentGroup;
  category: PublicTournamentCategory;
  pairs: readonly PublicTournamentPair[];
  /** The tournament's matches: the ones of this group are listed. */
  matches: readonly PublicTournamentMatch[];
}

/** One group: its standings table, the pairs that qualify marked, and its matches by matchday. */
export function GroupSection({ group, category, pairs, matches }: GroupSectionProps) {
  const lookup = pairLookup(pairs);
  const noGroupNames = new Map<string, string>();
  const title = `${t.groupTitlePrefix} ${group.name}`;

  return (
    <section aria-label={title} className="flex flex-col gap-3">
      <h4 className="text-text-primary text-base font-bold">{title}</h4>
      <StandingsTable group={group} category={category} lookup={lookup} />
      {category.qualifiers_per_group > 0 && (
        <p className="text-text-tertiary text-xs">{qualifiersNote(category.qualifiers_per_group)}</p>
      )}
      {groupMatchdays(matches, group.id).map((day) => (
        <div key={day.round} className="flex flex-col gap-2">
          <h5 className="text-text-secondary text-sm font-semibold">{`${t.matchdayPrefix} ${String(day.round)}`}</h5>
          <ul className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            {day.matches.map((match) => {
              const caption = matchCaption(match);
              return (
                <MatchLine
                  key={match.id}
                  match={match}
                  pairs={lookup}
                  groupNames={noGroupNames}
                  {...(caption ? { caption } : {})}
                />
              );
            })}
          </ul>
        </div>
      ))}
    </section>
  );
}
