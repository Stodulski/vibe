import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { eventsApi } from '../api/events.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CreateOpenPlayEventRequest } from '@/shared/types/api.types';

/**
 * Publishes an event. The error is not toasted: a 409 names the court and what
 * holds it, and the form shows that next to the fields to change (see
 * `eventConflictMessage`), so the owner reads it without it fading.
 */
export function useCreateEvent(complexId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateOpenPlayEventRequest) => eventsApi.create(complexId, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.events.base(complexId) });
      // The event now holds its courts: the grid and every availability read differ.
      void queryClient.invalidateQueries({ queryKey: queryKeys.courts.blockedSlotsBase(complexId) });
      toast.success(ES_AR.events.form.createSuccess);
    },
  });
}
