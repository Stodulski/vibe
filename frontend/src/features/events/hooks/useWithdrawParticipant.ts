import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { eventsApi } from '../api/events.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';

const t = ES_AR.events.withdraw;

/** The owner taking one participant out of an event: their spot frees up and what they paid comes back. */
export function useWithdrawParticipant(complexId: string, eventId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (registrationId: string) => eventsApi.withdraw(complexId, eventId, registrationId),
    onSuccess: ({ refund }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.events.base(complexId) });
      toast.success(t.success(refund.status === 'issued' || refund.status === 'queued'));
    },
    onError: (error: unknown) => {
      toast.error(t.error, { description: getHttpErrorMessage(error, '') || undefined });
    },
  });
}
