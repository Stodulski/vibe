import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { toast } from 'sonner';
import { publicBookingApi } from '../api/public-booking.api';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { queryKeys } from '@/shared/lib/queryKeys';

const t = ES_AR;

export function useCancelBookingFlow(token: string) {
  const [confirmOpen, setConfirmOpen] = useState(false);

  const {
    data: cancelInfo,
    isLoading: infoLoading,
    error: cancelInfoError,
    refetch: refetchCancelInfo,
  } = useQuery({
    queryKey: queryKeys.cancelInfo.byToken(token),
    queryFn: ({ signal }) => publicBookingApi.getCancelInfo(token, signal),
    enabled: !!token,
    retry: false,
    // The token is single-use and the page is a one-shot visit from an email:
    // nothing about this answer changes while the tab is open, and refetching
    // it would only risk re-reading a link the cancel below just spent.
    staleTime: Infinity,
  });

  // resolveLink (internal/bookings/public.go) is the whole authorization for
  // this route: 404 means the token never existed, 410 means it resolved but
  // the link is no longer live. Two different situations for the person
  // holding the link — collapsing them into one generic failure was what let
  // an expired link fall through into an empty cancel form.
  const linkExpired = cancelInfoError instanceof HTTPError && cancelInfoError.response.status === 410;
  const linkNotFound = cancelInfoError instanceof HTTPError && cancelInfoError.response.status === 404;

  const cancel = useMutation({
    mutationFn: () => publicBookingApi.cancelBooking({ token }),
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.publicBooking.cancelBookingError));
    },
  });

  return {
    // `refund` is the server's own state machine (`data.RefundResult`); render
    // its `message` verbatim rather than re-deriving copy from `refunded`.
    // Derived straight from the mutation's own `data` instead of mirrored
    // into a second `useState` that could disagree with it.
    result: cancel.data?.refund ?? null,
    confirmOpen,
    openConfirm: () => {
      setConfirmOpen(true);
    },
    closeConfirm: () => {
      setConfirmOpen(false);
    },
    confirmCancel: () => {
      cancel.mutate();
    },
    cancelInfo,
    infoLoading,
    cancelInfoError,
    refetchCancelInfo,
    linkExpired,
    linkNotFound,
    isCancelling: cancel.isPending,
  };
}
