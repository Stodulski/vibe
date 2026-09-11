import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { bookingsApi } from '../api/bookings.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage, getHttpStatus } from '@/shared/lib/utils';
import type { CreateBookingRequest } from '@/shared/types/api.types';

export function useCreateBooking(complexId: string) {
  const t = ES_AR;
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateBookingRequest) => bookingsApi.create(complexId, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.bookings.byComplex(complexId),
      });
      toast.success(t.bookings.createSuccess);
    },
    onError: (error: unknown) => {
      if (getHttpStatus(error) === 409) {
        // The server now refuses several distinct slot problems (no price
        // configured, outside schedule, closed that day, etc.) as English
        // prose with no code attached — see internal/bookings/grid.go and
        // create.go in backend. The client can't map prose to Spanish
        // without string-matching sentences that may be reworded any time,
        // so it owns the headline and surfaces the server's sentence
        // verbatim as detail instead of guessing which case happened.
        toast.error(t.bookings.createError, {
          description: getHttpErrorMessage(error, t.bookings.slotOccupied),
        });
        return;
      }
      toast.error(getHttpErrorMessage(error, t.bookings.createError));
    },
  });
}
