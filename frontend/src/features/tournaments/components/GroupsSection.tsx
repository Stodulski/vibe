import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { pairName } from '../lib/tournamentView';
import type { TournamentGroup, TournamentOwnerPair, TournamentStanding } from '@/shared/types/api.types';

const t = ES_AR.tournaments.groups;

const HEAD_CELL = 'text-text-tertiary px-2 py-2 text-xs font-medium';

const COLUMNS = ['position', 'pair', 'played', 'won', 'lost', 'sets', 'games'] as const;

/** Fixed widths so every zone lines its columns up with the others, whatever its pairs are called. */
const COLUMN_WIDTHS: Record<(typeof COLUMNS)[number], string> = {
  position: 'w-12',
  pair: '',
  played: 'w-12',
  won: 'w-12',
  lost: 'w-12',
  sets: 'w-16',
  games: 'w-20',
};

function GroupHead() {
  return (
    <>
      <colgroup>
        {COLUMNS.map((column) => (
          <col key={column} className={COLUMN_WIDTHS[column]} />
        ))}
      </colgroup>
      <thead>
        <tr className="border-border-subtle border-b text-left">
          {COLUMNS.map((column) => (
            <th
              key={column}
              scope="col"
              className={cn(HEAD_CELL, column !== 'position' && column !== 'pair' && 'text-center')}
            >
              {t.columns[column]}
            </th>
          ))}
        </tr>
      </thead>
    </>
  );
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
  const figure = 'score-text px-2 py-2.5 text-center';
  return (
    <tr className={cn('border-border-subtle border-b last:border-b-0', qualifies && 'bg-success-bg/30')}>
      <td className="score-text px-2 py-2.5">{row.rank}</td>
      <td className="min-w-0 px-2 py-2.5">
        <span className="text-text-primary block break-words">{pair ? pairName(pair) : ''}</span>
        {qualifies && <span className="text-success-text text-xs">{t.qualifies}</span>}
      </td>
      <td className={figure}>{row.played}</td>
      <td className={figure}>{row.won}</td>
      <td className={figure}>{row.lost}</td>
      <td className={figure}>{`${String(row.sets_for)}-${String(row.sets_against)}`}</td>
      <td className={figure}>{`${String(row.games_for)}-${String(row.games_against)}`}</td>
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
  return (
    <section className="space-y-2">
      <h3 className="text-text-primary text-sm font-semibold">{t.zone(group.name)}</h3>
      <div className="border-border-subtle overflow-x-auto rounded-xl border">
        <table aria-label={t.table(group.name)} className="w-full min-w-[26rem] table-fixed text-sm">
          <GroupHead />
          <tbody>
            {group.standings.map((row) => {
              const pair = pairs.get(row.pair_id);
              return (
                <StandingRow
                  key={row.pair_id}
                  row={row}
                  {...(pair ? { pair } : {})}
                  qualifies={row.rank <= qualifiers}
                />
              );
            })}
          </tbody>
        </table>
      </div>
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
  if (groups.length === 0) return <p className="text-text-tertiary text-sm">{t.none}</p>;
  return (
    <div className="space-y-5">
      {groups.map((group) => (
        <GroupTable key={group.id} group={group} pairs={pairs} qualifiers={qualifiers} />
      ))}
    </div>
  );
}
