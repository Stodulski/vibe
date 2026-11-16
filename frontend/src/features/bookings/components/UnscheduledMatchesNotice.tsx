import { Link } from 'react-router-dom';
import { CalendarClock } from 'lucide-react';
import { useTournaments } from '@/features/tournaments/hooks/useTournaments';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR.bookings.unscheduledMatches;

/**
 * Above the grid of a day that falls within a tournament: one line per active
 * tournament that still has matches with both pairs and no court or time, and
 * a link to its page to schedule them. It asks for the tournaments overlapping
 * that single day (the same request, and cache entry, the tournaments page
 * makes) and draws nothing while it loads, when the request fails, or when
 * nothing is left to schedule, so it never takes space from the grid it sits
 * over.
 */
export function UnscheduledMatchesNotice({ complexId, date }: { complexId: string; date: string }) {
  const { data } = useTournaments(complexId, date, date);
  const pending = (data ?? []).filter(
    (tournament) => tournament.status === 'active' && tournament.unscheduled_count > 0,
  );
  if (pending.length === 0) return null;

  return (
    <ul className="mb-4 space-y-2">
      {pending.map((tournament) => (
        <li
          key={tournament.id}
          className="border-warning-border bg-warning-bg flex items-center justify-between gap-3 rounded-xl border px-3.5 py-2.5 text-sm"
        >
          <span className="text-text-primary flex min-w-0 items-center gap-2">
            <CalendarClock className="text-warning-icon size-4 shrink-0" aria-hidden="true" />
            <span className="min-w-0 break-words">{t.message(tournament.name, tournament.unscheduled_count)}</span>
          </span>
          <Button asChild variant="outline" size="xs" className="shrink-0">
            <Link to={`/events/tournaments/${tournament.id}`}>{t.schedule}</Link>
          </Button>
        </li>
      ))}
    </ul>
  );
}
