import { formatEventDay } from '@/shared/lib/eventDates';
import { ES_AR } from '@/shared/i18n/es_AR';
import { Button } from '@/shared/components/ui/button';
import { cn, formatTime } from '@/shared/lib/utils';
import { MatchCourtFee, type CourtFeeActions } from './MatchCourtFee';
import { isCorrectionLocked } from '../lib/correctionLock';
import { bracketRounds, resultText, roundName, slotLabel } from '../lib/tournamentView';
import type { TournamentOwnerMatch, TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.matches;

interface Lookups {
  pairs: Map<string, TournamentOwnerPair>;
  groupNames: Map<string, string>;
}

interface MatchActions {
  onSchedule: (matchId: string) => void;
  onUnschedule: (matchId: string) => void;
  onResult: (matchId: string) => void;
}

function whenText(match: TournamentOwnerMatch): string {
  const schedule = match.schedule;
  if (!schedule) return t.unscheduled;
  return t.scheduledAt(schedule.court_name, formatEventDay(schedule.date), formatTime(schedule.start_time));
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
        className="rounded-lg"
        disabled={locked}
        aria-describedby={locked ? `locked-${match.id}` : undefined}
        onClick={() => {
          actions.onResult(match.id);
        }}
      >
        {match.status === 'completed' ? t.correctResult : t.loadResult}
      </Button>
      {locked && (
        <span id={`locked-${match.id}`} className="text-text-tertiary self-center text-xs">
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
  return (
    <div className="flex flex-wrap gap-2">
      {!done && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="rounded-lg"
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
          className="rounded-lg"
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
        <span className="text-text-tertiary self-center text-xs">{t.notReady}</span>
      )}
    </div>
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
  const winner = match.winner_pair_id;
  return (
    <li className="border-border-subtle space-y-2 border-b py-3 last:border-b-0">
      <p className="text-sm break-words">
        <span
          className={cn(
            winner && winner === match.pair1_id ? 'text-text-primary font-semibold' : 'text-text-secondary',
          )}
        >
          {name(1)}
        </span>
        <span className="text-text-tertiary"> {t.versus} </span>
        <span
          className={cn(
            winner && winner === match.pair2_id ? 'text-text-primary font-semibold' : 'text-text-secondary',
          )}
        >
          {name(2)}
        </span>
      </p>
      {score && <p className="text-text-primary score-text text-sm font-medium">{score}</p>}
      <p className="text-text-tertiary text-xs break-words first-letter:uppercase">{whenText(match)}</p>
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
    title: roundName(index + 1, bracketRounds(matches).length),
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
  if (blocks.length === 0) return <p className="text-text-tertiary text-sm">{t.none}</p>;
  return (
    <div className="space-y-5">
      {blocks.map((block) => (
        <section key={block.key} aria-label={block.title}>
          <h3 className="text-text-primary mb-1 text-sm font-semibold">{block.title}</h3>
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
