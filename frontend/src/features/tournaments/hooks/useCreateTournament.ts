import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { tournamentsApi } from '../api/tournaments.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CreateTournamentRequest } from '@/shared/types/api.types';

/** Publishes a tournament with its categories. The error is shown by the form, not toasted. */
export function useCreateTournament(complexId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateTournamentRequest) => tournamentsApi.create(complexId, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tournaments.base(complexId) });
      toast.success(ES_AR.tournaments.form.createSuccess);
    },
  });
}
