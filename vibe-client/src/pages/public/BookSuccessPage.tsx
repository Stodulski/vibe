import { z } from 'zod';
import { useParams, useLocation, useSearchParams, useNavigate } from 'react-router-dom';
import {
  useBookingStatus,
  BookingConfirmed,
  bookingInfoSchema,
  resolveBookingStatusView,
  readStoredBookingInfo,
} from '@/features/public-booking';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

// `location.state` is `history.state`: it survives a refresh and can carry
// whatever an earlier navigation put there, so it is validated rather than
// cast in with `as`.
const successStateSchema = z.object({
  token: z.string().min(1),
  bookingInfo: bookingInfoSchema.optional(),
});

export default function BookSuccessPage() {
  usePageTitle(t.publicBooking.bookingSuccess);
  const { slug } = useParams<{ slug: string }>();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();

  const parsedState = successStateSchema.safeParse(location.state);
  const state = parsedState.success ? parsedState.data : null;

  // token comes from state (no-deposit flow) or searchParams (MP redirect)
  const token = state?.token ?? searchParams.get('token') ?? null;

  // bookingInfo: from state (direct navigation) or sessionStorage (MP redirect)
  const bookingInfo = state?.bookingInfo ?? readStoredBookingInfo();

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
    void navigate(`/${String(slug)}`, { replace: true });
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
        slug={String(slug)}
        onRetry={handleRetry}
        onStatusRetry={() => {
          void refetch();
        }}
      />
    </div>
  );
}
