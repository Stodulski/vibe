import { useEffect, useState } from 'react';
import { BookingForm } from '../components/BookingForm';
import { BookingSummaryCard } from '../components/booking-form/BookingSummaryCard';
import { computeBookingPricing } from '../components/booking-form/pricing';
import { FormErrorAlert } from '../components/booking-form/FormErrorAlert';
import { StepIndicator } from '../components/StepIndicator';
import { CheckoutLayout } from '../components/CheckoutLayout';
import { LoadingSpinner } from '@vibe/ui';
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
  // taken on it, parked in sessionStorage for this slug. It is read when the
  // page mounts, and again when the browser shows this page from its
  // back/forward cache (`useSlotHold`), since another slot may have been held
  // meanwhile. A missing or invalid draft, or one without a hold (a direct
  // visit, a stale tab), sends the person back to the complex page.
  const [draft, setDraft] = useState(() => readConfirmDraft(slug));

  useEffect(() => {
    if (!draft) {
      navigateTo(publicComplexPath(slug), { replace: true });
    }
  }, [draft, slug]);

  if (!draft) {
    return null;
  }

  // Keyed by the hold: a draft that holds another slot starts the page over,
  // countdown, form and the life of the hold included.
  return <BookConfirmPageContent key={draft.holdToken} slug={slug} draft={draft} onDraftReplaced={setDraft} />;
}

interface BookConfirmPageContentProps {
  slug: string;
  draft: ConfirmDraft;
  onDraftReplaced: (draft: ConfirmDraft) => void;
}

function BookConfirmPageContent({ slug, draft, onDraftReplaced }: BookConfirmPageContentProps) {
  const hold = useSlotHold(slug, draft, onDraftReplaced);
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
    // Back to slot selection gives the held slot back, so the back link is an action, not a link.
    <CheckoutLayout
      currentStep={2}
      back={{ label: t.publicBooking.changeTimeSlot, onClick: hold.changeTime }}
      aside={<HoldCountdown expiresAt={draft.holdExpiresAt} onExpire={hold.expire} />}
      summary={<BookingSummaryCard slotInfo={draft} pricing={computeBookingPricing(draft)} bare />}
    >
      <BookingForm slotInfo={draft} onSubmit={handleSubmit} isLoading={isLoading} />

      {paymentLinkError && (
        <FormErrorAlert message={t.publicBooking.paymentLinkError} onRetry={retry} retryDisabled={isLoading} />
      )}
    </CheckoutLayout>
  );
}
