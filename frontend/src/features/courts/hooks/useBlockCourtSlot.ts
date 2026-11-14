import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { courtsApi } from '../api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { isFixedBookingConflict } from '../lib/fixedBookingConflict';
import type { BlockSlotRequest } from '@/shared/types/api.types';

export function useBlockCourtSlot(complexId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ courtId, data }: { courtId: string; data: BlockSlotRequest }) =>
      courtsApi.blockSlot(complexId, courtId, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.bookings.byComplex(complexId) });
      void queryClient.invalidateQueries({
        queryKey: queryKeys.courts.blockedSlotsBase(complexId),
        exact: false,
      });
      toast.success(ES_AR.courts.blockSuccess);
    },
    onError: (error: unknown) => {
      // The server names it in English; the owner needs to know what to do about it.
      if (isFixedBookingConflict(error)) {
        toast.error(ES_AR.bookings.series.slotTaken);
        return;
      }
      toast.error(getHttpErrorMessage(error, ES_AR.courts.blockError));
    },
  });
}
