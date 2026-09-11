import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { courtsApi } from '../api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

export function useDeleteBlockedSlot(complexId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (slotId: string) => courtsApi.deleteBlockedSlot(complexId, slotId),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.courts.blockedSlotsBase(complexId),
        exact: false,
      });
      void queryClient.invalidateQueries({
        queryKey: queryKeys.bookings.byComplex(complexId),
      });
      toast.success(ES_AR.courts.unblockSlotSuccess);
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, ES_AR.courts.unblockSlotError));
    },
  });
}
