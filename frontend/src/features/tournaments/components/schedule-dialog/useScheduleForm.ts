import { zodResolver } from '@hookform/resolvers/zod';
import { useAppForm } from '@/shared/lib/form';
import { useScheduleMatch } from '../../hooks/useScheduleMatch';
import { scheduleConflictMessage, scheduleErrorMessage } from '../../lib/tournamentErrors';
import { makeScheduleFormSchema, type ScheduleFormValues } from '../../schemas/scheduleForm.schema';
import type { Tournament, TournamentCategory, TournamentMatch } from '@/shared/types/api.types';

interface Options {
  complexId: string;
  tournament: Tournament;
  category: TournamentCategory;
  match: TournamentMatch;
  onDone: () => void;
}

/**
 * The schedule form. A match already scheduled opens with its court, day, time
 * and duration; a new one with the category's match duration. A 409 is kept as
 * `conflict`, in Spanish, for the dialog to show beside the fields.
 */
export function useScheduleForm({ complexId, tournament, category, match, onDone }: Options) {
  const schedule = useScheduleMatch(complexId, tournament.id);
  const current = match.schedule;
  const form = useAppForm<ScheduleFormValues>({
    resolver: zodResolver(makeScheduleFormSchema(tournament.start_date, tournament.end_date)),
    defaultValues: {
      court_id: current?.court_id ?? '',
      date: current?.date ?? '',
      start_time: current?.start_time ?? '',
      duration_minutes: current?.duration_minutes ?? category.match_duration_minutes,
    },
  });

  const onSubmit = (data: ScheduleFormValues) => {
    schedule.mutate({ matchId: match.id, data }, { onSuccess: onDone });
  };

  return {
    form,
    onSubmit,
    isPending: schedule.isPending,
    error: schedule.error,
    conflict: scheduleConflictMessage(schedule.error),
    errorMessage: schedule.error ? scheduleErrorMessage(schedule.error) : null,
  };
}
