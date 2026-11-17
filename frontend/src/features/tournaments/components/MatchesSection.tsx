import { keepTogether } from '@/shared/lib/keepTogether';
import { formatEventDayShort } from '@/shared/lib/eventDates';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatTime } from '@/shared/lib/utils';
import { OverflowMenu, type OverflowMenuItem } from '@/shared/components/common/OverflowMenu';
import { MatchCourtFee, type CourtFeeActions } from './MatchCourtFee';
import { MatchNames } from './MatchNames';
import { isCorrectionLocked } from '../lib/correctionLock';
import {
  bracketRounds,
  courtFeeSides,
  matchSide,
  resultText,
  roundName,
  slotLabel,
  winnerSide,
} from '../lib/tournamentView';
import type { TournamentOwnerMatch, TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.matches;
const fee = ES_AR.tournaments.courtFee;

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

/** Charge each pair that owes the court fee of the match, or undo the charge of each pair that paid it. */
function courtFeeItems(match: TournamentOwnerMatch, lookups: Lookups, actions: CourtFeeActions): OverflowMenuItem[] {
  return courtFeeSides(match).map((side) => {
    const name = slotLabel(match, side.side, lookups.pairs, lookups.groupNames);
    const target = { matchId: match.id, side: side.side };
    return side.charged
      ? {
          kind: 'action',
          key: `undo-${String(side.side)}`,
          label: fee.undoAria(name),
          onClick: () => {
            actions.onUndo(target);
          },
        }
      : {
          kind: 'action',
          key: `charge-${String(side.side)}`,
          label: fee.chargeAria(name),
          onClick: () => {
            actions.onCharge(target);
          },
        };
  });
}

/**
 * The actions a match's menu offers, in order: scheduling and the result while the
 * match can still change, then the court fee of each pair. A played match that can
 * no longer be corrected offers no scheduling or result, but its court fee stays.
 */
function matchMenuItems(
  match: TournamentOwnerMatch,
  locked: boolean,
  lookups: Lookups,
  actions: MatchActions,
  courtFeeActions: CourtFeeActions,
): OverflowMenuItem[] {
  const done = match.status === 'completed';
  const ready = !!match.pair1_id && !!match.pair2_id;
  const items: OverflowMenuItem[] = [];
  if (!done) {
    items.push({
      kind: 'action',
      key: 'schedule',
      label: match.schedule ? t.reschedule : t.schedule,
      onClick: () => {
        actions.onSchedule(match.id);
      },
    });
  }
  if (!done && match.schedule) {
    items.push({
      kind: 'action',
      key: 'unschedule',
      label: t.unschedule,
      onClick: () => {
        actions.onUnschedule(match.id);
      },
    });
  }
  if (ready && !locked) {
    items.push({
      kind: 'action',
      key: 'result',
      label: done ? t.correctResult : t.loadResult,
      onClick: () => {
        actions.onResult(match.id);
      },
    });
  }
  return [...items, ...courtFeeItems(match, lookups, courtFeeActions)];
}

function MatchRow({
  match,
  lookups,
  actions,
  courtFee,
  editable,
  locked,
}: {
  match: TournamentOwnerMatch;
  lookups: Lookups;
  actions: MatchActions;
  courtFee: { fee: number; actions: CourtFeeActions };
  editable: boolean;
  locked: boolean;
}) {
  const score = resultText(match, lookups.pairs);
  const ready = !!match.pair1_id && !!match.pair2_id;
  const when = whenText(match);
  const items = editable ? matchMenuItems(match, locked, lookups, actions, courtFee.actions) : [];
  return (
    <li className="border-border-subtle border-b py-3 last:border-b-0">
      <div className="flex min-w-0 items-start gap-2">
        <div className="min-w-0 flex-1 space-y-2">
          <MatchNames
            first={matchSide(match, 1, lookups.pairs, lookups.groupNames)}
            second={matchSide(match, 2, lookups.pairs, lookups.groupNames)}
            winnerSide={winnerSide(match)}
          />
          {score && <p className="text-text-primary truncate text-sm font-semibold tabular-nums">{score}</p>}
          <p className="text-text-tertiary truncate text-xs first-letter:uppercase" title={when}>
            {when}
          </p>
          {editable && !ready && (
            <p className="text-text-tertiary truncate text-xs" title={t.notReady}>
              {t.notReady}
            </p>
          )}
          <MatchCourtFee match={match} fee={courtFee.fee} pairs={lookups.pairs} groupNames={lookups.groupNames} />
        </div>
        <OverflowMenu label={t.actionsLabel} items={items} />
      </div>
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

/** Every match with its court and time, and one menu per match with the actions it still offers. */
export function MatchesSection({ matches, pairs, groupNames, actions, courtFee, editable }: MatchesSectionProps) {
  const blocks = blocksOf(matches, groupNames);
  if (blocks.length === 0) return <p className="text-text-tertiary text-xs">{t.none}</p>;
  return (
    <div className="space-y-8">
      {blocks.map((block) => (
        <section key={block.key} aria-label={block.title}>
          <h3 className="text-primary-400 mb-1 truncate text-base font-bold">{block.title}</h3>
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
              />
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
