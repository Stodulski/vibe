import { keepTogether } from '@/shared/lib/keepTogether';
import { formatEventDayShort } from '@/shared/lib/eventDates';
import { ES_AR } from '@/shared/i18n/es_AR';
import { Button } from '@/shared/components/ui/button';
import { cn, formatTime } from '@/shared/lib/utils';
import { MatchCourtFee, type CourtFeeActions } from './MatchCourtFee';
import { isCorrectionLocked } from '../lib/correctionLock';
import { bracketRounds, resultText, roundName, slotLabel } from '../lib/tournamentView';
import type { TournamentOwnerMatch, TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.matches;

/** Every action button of a match row: one line of label, smaller below `sm` so two of them sit side by side at 320 px. */
const ACTION_BUTTON = 'rounded-lg whitespace-nowrap px-2.5 text-xs sm:px-3 sm:text-sm';

interface Lookups {
  pairs: Map<string, TournamentOwnerPair>;
  groupNames: Map<string, string>;
}

interface MatchActions {
  onSchedule: (matchId: string) => void;
  onUnschedule: (matchId: string) => void;
  onResult: (matchId: string) => void;
}

/** "Pádel 1 · Dom 22/11 · 09:00": the court, the day in figures and the time, on one line. */
function whenText(match: TournamentOwnerMatch): string {
  const schedule = match.schedule;
  if (!schedule) return t.unscheduled;
  return t.scheduledAt(
    keepTogether(schedule.court_name),
    formatEventDayShort(schedule.date),
    formatTime(schedule.start_time),
  );
}

function ResultButton({
  match,
  actions,
  locked,
  primary,
}: {
  match: TournamentOwnerMatch;
  actions: MatchActions;
  locked: boolean;
  /** The one match whose next action is the primary of the view. */
  primary: boolean;
}) {
  return (
    <>
      <Button
        type="button"
        size="sm"
        variant={primary ? 'default' : 'outline'}
        className={ACTION_BUTTON}
        disabled={locked}
        aria-describedby={locked ? `locked-${match.id}` : undefined}
        onClick={() => {
          actions.onResult(match.id);
        }}
      >
        {match.status === 'completed' ? t.correctResult : t.loadResult}
      </Button>
      {locked && (
        <span
          id={`locked-${match.id}`}
          className="text-text-tertiary max-w-full min-w-0 self-center truncate text-xs"
          title={t.correctLocked}
        >
          {t.correctLocked}
        </span>
      )}
    </>
  );
}

function MatchButtons({
  match,
  actions,
  ready,
  locked,
  primary,
}: {
  match: TournamentOwnerMatch;
  actions: MatchActions;
  ready: boolean;
  /** The played match cannot be corrected: what comes next already has a result. */
  locked: boolean;
  primary: boolean;
}) {
  const done = match.status === 'completed';
  // A played match that can no longer be corrected has nothing to act on: no greyed
  // button and no note repeated on every row.
  if (done && locked) return null;
  return (
    <div className="flex flex-wrap gap-2">
      {!done && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className={ACTION_BUTTON}
          onClick={() => {
            actions.onSchedule(match.id);
          }}
        >
          {match.schedule ? t.reschedule : t.schedule}
        </Button>
      )}
      {!done && match.schedule && (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className={ACTION_BUTTON}
          onClick={() => {
            actions.onUnschedule(match.id);
          }}
        >
          {t.unschedule}
        </Button>
      )}
      {ready ? (
        <ResultButton match={match} actions={actions} locked={locked} primary={primary} />
      ) : (
        <span className="text-text-tertiary max-w-full min-w-0 self-center truncate text-xs" title={t.notReady}>
          {t.notReady}
        </span>
      )}
    </div>
  );
}

/** Which side won, if the match has a winner. */
function winnerSide(match: TournamentOwnerMatch): 1 | 2 | null {
  if (!match.winner_pair_id) return null;
  if (match.winner_pair_id === match.pair1_id) return 1;
  return match.winner_pair_id === match.pair2_id ? 2 : null;
}

/** The two pairs: stacked on phones with a large green "vs" between them, one line from sm. */
function MatchNames({ first, second, winnerSide }: { first: string; second: string; winnerSide: 1 | 2 | null }) {
  const pairClass = (side: 1 | 2) =>
    cn('min-w-0 truncate', winnerSide === side ? 'text-text-primary font-semibold' : 'text-text-secondary');
  return (
    <p className="flex w-fit max-w-full min-w-0 flex-col text-sm sm:flex-row" title={`${first} ${t.versus} ${second}`}>
      <span className={pairClass(1)}>{first}</span>
      <span className="text-primary-400 shrink-0 text-base font-bold sm:px-1.5 sm:text-sm sm:font-semibold">
        {t.versus}
      </span>
      <span className={pairClass(2)}>{second}</span>
    </p>
  );
}

function MatchRow({
  match,
  lookups,
  actions,
  courtFee,
  editable,
  locked,
  primary,
}: {
  match: TournamentOwnerMatch;
  lookups: Lookups;
  actions: MatchActions;
  courtFee: { fee: number; actions: CourtFeeActions };
  editable: boolean;
  locked: boolean;
  primary: boolean;
}) {
  const score = resultText(match, lookups.pairs);
  const ready = !!match.pair1_id && !!match.pair2_id;
  const name = (side: 1 | 2) => slotLabel(match, side, lookups.pairs, lookups.groupNames);
  const when = whenText(match);
  return (
    <li className="border-border-subtle space-y-2 border-b py-3 last:border-b-0">
      <MatchNames first={name(1)} second={name(2)} winnerSide={winnerSide(match)} />
      {score && <p className="text-text-primary truncate text-sm font-semibold tabular-nums">{score}</p>}
      <p className="text-text-tertiary truncate text-xs first-letter:uppercase" title={when}>
        {when}
      </p>
      <MatchCourtFee
        match={match}
        fee={courtFee.fee}
        pairs={lookups.pairs}
        groupNames={lookups.groupNames}
        actions={courtFee.actions}
        editable={editable}
      />
      {editable && <MatchButtons match={match} actions={actions} ready={ready} locked={locked} primary={primary} />}
    </li>
  );
}

interface Block {
  key: string;
  title: string;
  matches: TournamentOwnerMatch[];
}

/** The matches in the order they are played: each group's matchdays, then the bracket round by round. Byes are left out. */
function blocksOf(matches: TournamentOwnerMatch[], groupNames: Map<string, string>): Block[] {
  const real = matches.filter((match) => !match.bye);
  const groupBlocks = [...groupNames.entries()].map(([id, name]) => ({
    key: `group-${id}`,
    title: ES_AR.tournaments.groups.zone(name),
    matches: real
      .filter((match) => match.stage === 'group' && match.group_id === id)
      .sort((a, b) => a.round - b.round || a.position - b.position),
  }));
  const rounds = bracketRounds(real);
  const bracketBlocks = rounds.map((round, index) => ({
    key: `round-${String(index)}`,
    title: roundName(index + 1, bracketRounds(matches).length, round.length),
    matches: round,
  }));
  return [...groupBlocks, ...bracketBlocks].filter((block) => block.matches.length > 0);
}

interface MatchesSectionProps extends Lookups {
  matches: TournamentOwnerMatch[];
  actions: MatchActions;
  /** The category's current court fee per pair (centavos) and the actions to charge or undo it. */
  courtFee: { fee: number; actions: CourtFeeActions };
  /** False once the tournament or category is cancelled: matches are read only. */
  editable: boolean;
}

/** Every match with its court and time, and the actions to schedule it and load its result. */
export function MatchesSection({ matches, pairs, groupNames, actions, courtFee, editable }: MatchesSectionProps) {
  const blocks = blocksOf(matches, groupNames);
  // The next match waiting for a result, in playing order: the only primary button of the view.
  const nextId = blocks
    .flatMap((block) => block.matches)
    .find((match) => match.status !== 'completed' && !!match.pair1_id && !!match.pair2_id)?.id;
  if (blocks.length === 0) return <p className="text-text-tertiary text-xs">{t.none}</p>;
  return (
    <div className="space-y-5">
      {blocks.map((block) => (
        <section key={block.key} aria-label={block.title}>
          <h3 className="text-text-primary mb-1 truncate text-sm font-semibold" title={block.title}>
            {block.title}
          </h3>
          <ul>
            {block.matches.map((match) => (
              <MatchRow
                key={match.id}
                match={match}
                lookups={{ pairs, groupNames }}
                actions={actions}
                courtFee={courtFee}
                editable={editable}
                locked={match.status === 'completed' && isCorrectionLocked(match, matches)}
                primary={match.id === nextId}
              />
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
