import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { eventsApi } from '../api/events.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';

const t = ES_AR.events.cancel;

export function useCancelEvent(complexId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (eventId: string) => eventsApi.cancel(complexId, eventId),
    onSuccess: ({ refunds }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.events.base(complexId) });
      // The courts are free again, so the grid's event blocks go away.
      void queryClient.invalidateQueries({ queryKey: queryKeys.courts.blockedSlotsBase(complexId) });
      const owed = refunds.filter(({ refund }) => refund.status === 'issued' || refund.status === 'queued').length;
      toast.success(t.success(owed));
    },
    onError: (error: unknown) => {
      toast.error(t.error, { description: getHttpErrorMessage(error, '') || undefined });
    },
  });
}
