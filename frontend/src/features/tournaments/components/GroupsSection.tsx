import { Check } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { pairName } from '../lib/tournamentView';
import type { TournamentGroup, TournamentOwnerPair, TournamentStanding } from '@/shared/types/api.types';

const t = ES_AR.tournaments.groups;

const HEAD_CELL = 'text-text-tertiary px-1.5 py-2 text-xs font-medium';
const FIGURE = 'score-text px-1.5 py-2.5 text-center';

/**
 * The columns, in order, with the width each one keeps. Fixed widths line every
 * zone's columns up with the others, whatever its pairs are called. A phone keeps
 * position, pair, wins and the set difference; from `sm` the table shows every
 * figure and drops the difference.
 */
const COLUMNS = [
  { key: 'position', width: 'w-10', visibility: '' },
  { key: 'pair', width: '', visibility: '' },
  { key: 'played', width: 'w-10', visibility: 'hidden sm:table-cell' },
  { key: 'won', width: 'w-10', visibility: '' },
  { key: 'lost', width: 'w-10', visibility: 'hidden sm:table-cell' },
  { key: 'sets', width: 'w-16', visibility: 'hidden sm:table-cell' },
  { key: 'games', width: 'w-20', visibility: 'hidden sm:table-cell' },
  { key: 'difference', width: 'w-12', visibility: 'sm:hidden' },
] as const;

function GroupHead() {
  return (
    <thead>
      <tr className="border-border-subtle border-b text-left">
        {COLUMNS.map(({ key, width, visibility }) => (
          <th
            key={key}
            scope="col"
            className={cn(HEAD_CELL, width, visibility, key !== 'position' && key !== 'pair' && 'text-center')}
          >
            {key === 'difference' ? <abbr title={t.differenceTitle}>{t.columns.difference}</abbr> : t.columns[key]}
          </th>
        ))}
      </tr>
    </thead>
  );
}

/** A set difference with its sign: "+2", "0", "-1". */
function signed(value: number): string {
  return value > 0 ? `+${String(value)}` : String(value);
}

function StandingRow({
  row,
  pair,
  qualifies,
}: {
  row: TournamentStanding;
  pair?: TournamentOwnerPair;
  qualifies: boolean;
}) {
  const name = pair ? pairName(pair) : '';
  return (
    <tr className={cn('border-border-subtle border-b last:border-b-0', qualifies && 'bg-success-bg/30')}>
      <td className="score-text px-1.5 py-2.5">{row.rank}</td>
      <td className="min-w-0 px-1.5 py-2.5">
        <span className="flex min-w-0 items-center gap-1.5">
          <span className="text-text-primary min-w-0 truncate" title={name || undefined}>
            {name}
          </span>
          {qualifies && (
            <>
              <Check className="text-success-text size-3.5 shrink-0" aria-hidden="true" />
              <span className="sr-only">{t.qualifies}</span>
            </>
          )}
        </span>
      </td>
      <td className={cn(FIGURE, 'hidden sm:table-cell')}>{row.played}</td>
      <td className={FIGURE}>{row.won}</td>
      <td className={cn(FIGURE, 'hidden sm:table-cell')}>{row.lost}</td>
      <td className={cn(FIGURE, 'hidden sm:table-cell')}>{`${String(row.sets_for)}-${String(row.sets_against)}`}</td>
      <td className={cn(FIGURE, 'hidden sm:table-cell')}>{`${String(row.games_for)}-${String(row.games_against)}`}</td>
      <td className={cn(FIGURE, 'sm:hidden')}>{signed(row.sets_for - row.sets_against)}</td>
    </tr>
  );
}

function GroupTable({
  group,
  pairs,
  qualifiers,
}: {
  group: TournamentGroup;
  pairs: Map<string, TournamentOwnerPair>;
  qualifiers: number;
}) {
  const title = t.zone(group.name);
  return (
    <section className="min-w-0 space-y-2">
      <h3 className="text-text-primary truncate text-sm font-semibold" title={title}>
        {title}
      </h3>
      <table aria-label={t.table(group.name)} className="w-full table-fixed text-sm">
        <GroupHead />
        <tbody>
          {group.standings.map((row) => {
            const pair = pairs.get(row.pair_id);
            return (
              <StandingRow key={row.pair_id} row={row} {...(pair ? { pair } : {})} qualifies={row.rank <= qualifiers} />
            );
          })}
        </tbody>
      </table>
    </section>
  );
}

interface GroupsSectionProps {
  groups: TournamentGroup[];
  pairs: Map<string, TournamentOwnerPair>;
  qualifiers: number;
}

/** One standings table per group, best first, marking the pairs that go on to the bracket. */
export function GroupsSection({ groups, pairs, qualifiers }: GroupsSectionProps) {
  if (groups.length === 0) return <p className="text-text-tertiary text-xs">{t.none}</p>;
  return (
    <div className="space-y-5">
      {groups.map((group) => (
        <GroupTable key={group.id} group={group} pairs={pairs} qualifiers={qualifiers} />
      ))}
    </div>
  );
}
