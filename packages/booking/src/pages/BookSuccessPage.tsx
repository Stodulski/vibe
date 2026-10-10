import { useMemo } from 'react';
import { BookingConfirmed } from '../components/BookingConfirmed';
import { resolveBookingStatusView } from '../components/booking-confirmed/statusView';
import { readStoredBookingInfo } from '../lib/storedBookingInfo';
import { useBookingStatus } from '../hooks/useBookingStatus';
import { publicComplexPath } from '../lib/publicPaths';
import { navigateTo, useQueryParams } from '../lib/navigation';
import { readBookingResult } from '../lib/handoff';

export default function BookSuccessPage({ slug }: { slug: string }) {
  const [searchParams] = useQueryParams();

  // The result is parked by the confirm page for this slug (no-deposit flow). It
  // is read once, at mount, so a refresh still shows the same booking.
  const result = useMemo(() => readBookingResult(slug), [slug]);

  // token comes from the result (no-deposit flow) or searchParams (MP redirect)
  const token = result?.token ?? searchParams.get('token') ?? null;

  // bookingInfo: from the result (no-deposit flow) or sessionStorage (MP redirect)
  const bookingInfo = result?.bookingInfo ?? readStoredBookingInfo();

  // MercadoPago redirects here with `?status=pending` when the payment
  // itself is still under review on their end (e.g. a cash voucher or bank
  // transfer) — a state that can take days, not the few seconds the normal
  // webhook lag takes. Read it so that case gets its own message instead of
  // eventually falling into the "processing" timeout copy, which promises
  // "a few minutes".
  const paymentUnderReview = searchParams.get('status') === 'pending';

  const {
    data: bookingStatus,
    isLoading,
    isError,
    timedOut,
    linkExpired,
    linkNotFound,
    refetch,
  } = useBookingStatus(token);

  const view = resolveBookingStatusView({
    status: bookingStatus?.status,
    collectionStatus: bookingStatus?.collection_status,
    isLoading,
    isError,
    timedOut,
    paymentUnderReview,
    linkExpired,
    linkNotFound,
  });

  function handleRetry() {
    navigateTo(publicComplexPath(slug), { replace: true });
  }

  return (
    <div className="w-full">
      <BookingConfirmed
        view={view}
        status={bookingStatus?.status}
        collectionStatus={bookingStatus?.collection_status}
        bookingInfo={bookingInfo}
        bookingDetails={bookingStatus}
        token={token}
        slug={slug}
        onRetry={handleRetry}
        onStatusRetry={() => {
          void refetch();
        }}
      />
    </div>
  );
}
