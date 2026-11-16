import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { tournamentsApi } from '../api/tournaments.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { ScheduleMatchRequest } from '@/shared/types/api.types';

/** Refetches the tournament (the match's hours) and the grid, which draws the court the match holds. */
function useRefreshAfterSchedule(complexId: string) {
  const queryClient = useQueryClient();
  return () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.tournaments.base(complexId) });
    void queryClient.invalidateQueries({ queryKey: queryKeys.courts.blockedSlotsBase(complexId) });
  };
}

/** Gives a match a court, a day and a time. A 409 names what holds the hours; the dialog shows it, so it is not toasted. */
export function useScheduleMatch(complexId: string, tournamentId: string) {
  const refresh = useRefreshAfterSchedule(complexId);
  return useMutation({
    mutationFn: ({ matchId, data }: { matchId: string; data: ScheduleMatchRequest }) =>
      tournamentsApi.schedule(complexId, tournamentId, matchId, data),
    onSuccess: () => {
      refresh();
      toast.success(ES_AR.tournaments.schedule.success);
    },
  });
}

export function useUnscheduleMatch(complexId: string, tournamentId: string) {
  const refresh = useRefreshAfterSchedule(complexId);
  return useMutation({
    mutationFn: (matchId: string) => tournamentsApi.unschedule(complexId, tournamentId, matchId),
    onSuccess: () => {
      refresh();
      toast.success(ES_AR.tournaments.schedule.removed);
    },
    onError: () => {
      toast.error(ES_AR.tournaments.schedule.unscheduleError);
    },
  });
}
