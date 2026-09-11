import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { bookingsApi } from '../api/bookings.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UpdateBookingRequest } from '@/shared/types/api.types';

export function useUpdateBooking(complexId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bookingId, data }: { bookingId: string; data: UpdateBookingRequest }) =>
      bookingsApi.update(complexId, bookingId, data),
    onSuccess: (_data, { bookingId }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.bookings.byComplex(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.bookings.detail(bookingId) });
      toast.success(ES_AR.bookings.updateSuccess);
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, ES_AR.bookings.updateError));
    },
  });
}
