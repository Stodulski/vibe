import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  usePublicBooking,
  BOOKING_INFO_KEY,
  type BookingSlotInfo,
  type PublicBookingFormData,
} from '@/features/public-booking';
import { buildBookingRequest } from './buildBookingRequest';
import { buildBookingInfo } from './buildBookingInfo';
import { safeSessionStorage } from '@/shared/lib/safeStorage';

export { BOOKING_INFO_KEY };

export function useConfirmBookingSubmit(slug: string | undefined, slotInfo: BookingSlotInfo) {
  const navigate = useNavigate();
  const [redirecting, setRedirecting] = useState(false);
  const [paymentLinkError, setPaymentLinkError] = useState(false);
  // Kept so "Reintentar" can resubmit exactly what the person already typed —
  // the whole point of the persistent error is that their data is not lost.
  const [lastFormData, setLastFormData] = useState<PublicBookingFormData | null>(null);

  const mutation = usePublicBooking({
    onConflict: () => {
      // Slot was taken — redirect back to slot selection with date pre-selected.
      void navigate(`/${String(slug)}?date=${slotInfo.date}`, { replace: true });
    },
    onPaymentLinkError: () => {
      setPaymentLinkError(true);
    },
  });

  function handleSubmit(formData: PublicBookingFormData) {
    setPaymentLinkError(false);
    setLastFormData(formData);
    mutation.mutate(buildBookingRequest(slotInfo, formData), {
      onSuccess: (data) => {
        const bookingInfo = buildBookingInfo(slotInfo, formData.client_phone);

        // Persist bookingInfo so it survives the MP redirect. Best-effort: in
        // Safari private browsing (or a full quota) the write fails silently
        // and the summary is lost, but the booking already exists on the
        // server; getting stuck on this screen with a blocked slot would be
        // worse.
        safeSessionStorage.set(BOOKING_INFO_KEY, JSON.stringify(bookingInfo));

        if (data.mp_init_point) {
          setRedirecting(true);
          window.location.href = data.mp_init_point;
        } else {
          void navigate(`/${String(slug)}/book/success`, {
            state: {
              token: data.token,
              bookingInfo,
            },
            replace: true,
          });
        }
      },
    });
  }

  function retry() {
    if (lastFormData) handleSubmit(lastFormData);
  }

  return { handleSubmit, isLoading: mutation.isPending, redirecting, paymentLinkError, retry };
}
