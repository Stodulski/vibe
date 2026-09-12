import { useParams, useLocation, useNavigate, Navigate } from 'react-router-dom';
import { ArrowLeft, AlertCircle } from 'lucide-react';
import { BookingForm, type BookingSlotInfo, bookingSlotInfoSchema, StepIndicator } from '@/features/public-booking';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { Button } from '@/shared/components/ui/button';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useConfirmBookingSubmit } from './book-confirm-page/useConfirmBookingSubmit';

const t = ES_AR;

export default function BookConfirmPage() {
  usePageTitle(t.publicBooking.confirmBooking);
  const { slug } = useParams<{ slug: string }>();
  const location = useLocation();
  const navigate = useNavigate();

  // `location.state` is `history.state`: it survives a refresh and can carry
  // whatever an earlier navigation put there, so it is validated rather than
  // cast. Anything that doesn't match — no state (direct navigation), a
  // stale/garbage shape — redirects back, same as the old "no state" check.
  const parsedSlotInfo = bookingSlotInfoSchema.safeParse(location.state);
  const slotInfo = parsedSlotInfo.success ? parsedSlotInfo.data : null;

  if (!slotInfo) {
    return <Navigate to={`/${String(slug)}`} replace />;
  }

  // Back lands on the last step taken, not the first: the query carries every
  // answer, and the storefront reopens with the court question at this hour.
  const back = new URLSearchParams({
    date: slotInfo.date,
    duration: String(slotInfo.durationMinutes),
    time: slotInfo.startTime,
  });
  if (slotInfo.sport) back.set('sport', slotInfo.sport);

  return (
    <BookConfirmPageContent
      slug={slug}
      slotInfo={slotInfo}
      onBack={() => {
        void navigate(`/${String(slug)}?${back.toString()}`, { replace: true });
      }}
    />
  );
}

interface BookConfirmPageContentProps {
  slug: string | undefined;
  slotInfo: BookingSlotInfo;
  onBack: () => void;
}

function BookConfirmPageContent({ slug, slotInfo, onBack }: BookConfirmPageContentProps) {
  const { handleSubmit, isLoading, redirecting, paymentLinkError, retry } = useConfirmBookingSubmit(slug, slotInfo);

  if (redirecting) {
    return (
      <div className="animate-fade-in flex w-full flex-1 flex-col items-center justify-center gap-5 py-16">
        <StepIndicator currentStep={3} />
        <LoadingSpinner size="lg" />
        <p className="text-text-secondary text-sm">{t.publicBooking.redirectingToMP}</p>
      </div>
    );
  }

  return (
    <div className="animate-fade-in w-full space-y-6 sm:space-y-8">
      <StepIndicator currentStep={2} />

      {/* Back to slot selection */}
      <button
        type="button"
        onClick={onBack}
        className="text-text-tertiary hover:text-text-secondary flex min-h-12 items-center gap-1.5 text-sm transition-colors sm:min-h-0"
      >
        <ArrowLeft className="size-3.5" />
        {t.publicBooking.changeTimeSlot}
      </button>

      <BookingForm slotInfo={slotInfo} onSubmit={handleSubmit} isLoading={isLoading} />

      {/* Stays on screen until the person acts, unlike the toast it replaces
          for this one case: the data they typed survived, and "Reintentar"
          resubmits it as-is instead of asking them to start over. */}
      {paymentLinkError && (
        <div
          role="alert"
          className="border-error-border/30 bg-error-bg text-error-text flex flex-col items-start gap-3 rounded-xl border p-4 text-sm sm:flex-row sm:items-center sm:justify-between"
        >
          <span className="flex items-start gap-2">
            <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            {t.publicBooking.paymentLinkError}
          </span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="border-error-border/30 text-error-text hover:bg-error-bg/50 min-h-12 w-full shrink-0 rounded-xl sm:min-h-0 sm:w-auto"
            onClick={retry}
          >
            {t.publicBooking.retryPaymentLink}
          </Button>
        </div>
      )}
    </div>
  );
}
