import { useEffect, useMemo } from 'react';
import { ArrowLeft, AlertCircle } from 'lucide-react';
import { BookingForm } from '../components/BookingForm';
import { StepIndicator } from '../components/StepIndicator';
import { LoadingSpinner } from '@vibe/ui';
import { Button } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../messages';
import { publicComplexPath } from '../lib/publicPaths';
import { useConfirmBookingSubmit } from './book-confirm-page/useConfirmBookingSubmit';
import { useSlotHold } from './book-confirm-page/useSlotHold';
import { HoldCountdown } from './book-confirm-page/HoldCountdown';
import { navigateTo } from '../lib/navigation';
import { readConfirmDraft, type ConfirmDraft } from '../lib/handoff';

const t = BOOKING_MESSAGES;

export default function BookConfirmPage({ slug }: { slug: string }) {
  // The draft is the slot the person chose on the complex page and the hold
  // taken on it, parked in sessionStorage for this slug. It is read once, when
  // the page mounts. A missing or invalid draft, or one without a hold (a
  // direct visit, a stale tab), sends the person back to the complex page.
  const draft = useMemo(() => readConfirmDraft(slug), [slug]);

  useEffect(() => {
    if (!draft) {
      navigateTo(publicComplexPath(slug), { replace: true });
    }
  }, [draft, slug]);

  if (!draft) {
    return null;
  }

  return <BookConfirmPageContent slug={slug} draft={draft} />;
}

interface BookConfirmPageContentProps {
  slug: string;
  draft: ConfirmDraft;
}

function BookConfirmPageContent({ slug, draft }: BookConfirmPageContentProps) {
  const hold = useSlotHold(slug, draft);
  const { handleSubmit, isLoading, redirecting, paymentLinkError, retry } = useConfirmBookingSubmit(slug, draft, {
    onExpired: hold.expire,
    onConverted: hold.markConverted,
  });

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

      {/* Back to slot selection, giving the held slot back */}
      <button
        type="button"
        onClick={hold.changeTime}
        className="text-text-tertiary hover:text-text-secondary flex min-h-12 items-center gap-1.5 text-sm transition-colors sm:min-h-0"
      >
        <ArrowLeft className="size-3.5" />
        {t.publicBooking.changeTimeSlot}
      </button>

      <HoldCountdown expiresAt={draft.holdExpiresAt} onExpire={hold.expire} />

      <BookingForm slotInfo={draft} onSubmit={handleSubmit} isLoading={isLoading} />

      {paymentLinkError && <PaymentLinkErrorAlert onRetry={retry} retryDisabled={isLoading} />}
    </div>
  );
}

/**
 * Stays on screen until the person acts, unlike the toast it replaces for this
 * one case: the data they typed survived, and "Reintentar" resubmits it as-is
 * instead of asking them to start over. Disabled while that resubmit runs.
 */
function PaymentLinkErrorAlert({ onRetry, retryDisabled }: { onRetry: () => void; retryDisabled: boolean }) {
  return (
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
        disabled={retryDisabled}
        onClick={onRetry}
      >
        {t.publicBooking.retryPaymentLink}
      </Button>
    </div>
  );
}
