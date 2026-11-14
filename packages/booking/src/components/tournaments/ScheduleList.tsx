import { formatDateFull } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import type { PublicTournamentMatch, PublicTournamentPair } from '../../api/types';
import { MatchLine } from './MatchLine';
import { matchStage, pairLookup, scheduleByDay } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

interface ScheduleListProps {
  /** The category's matches: the scheduled ones are listed, with their result once played. */
  matches: readonly PublicTournamentMatch[];
  pairs: readonly PublicTournamentPair[];
  groupNames: ReadonlyMap<string, string>;
}

/** The scheduled matches by day, each with its court, time and result. Nothing renders when none is scheduled. */
export function ScheduleList({ matches, pairs, groupNames }: ScheduleListProps) {
  const days = scheduleByDay(matches);
  if (days.length === 0) return null;
  const lookup = pairLookup(pairs);
  const rounds = Math.max(0, ...matches.filter((m) => m.stage === 'bracket').map((m) => m.round));

  return (
    <div className="flex flex-col gap-4">
      {days.map((day) => (
        <div key={day.date} className="flex flex-col gap-2">
          <h4 className="text-text-secondary text-sm font-semibold first-letter:uppercase">
            {formatDateFull(day.date)}
          </h4>
          <ul className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            {day.matches.map((match) => (
              <MatchLine
                key={match.id}
                match={match}
                pairs={lookup}
                groupNames={groupNames}
                caption={`${match.schedule?.start_time ?? ''} ${t.hoursSuffix}, ${match.schedule?.court_name ?? ''}. ${matchStage(match, groupNames, rounds)}`}
              />
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}
