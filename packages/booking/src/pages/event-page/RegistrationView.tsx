import { CheckCircle2 } from 'lucide-react';
import { Button, ConfirmDialog, StatusHero, formatDeadline, formatPrice } from '@vibe/ui';
import { useState } from 'react';
import { BOOKING_MESSAGES } from '../../messages';
import { useCancelEventRegistration } from '../../hooks/useEventRegistration';
import { EventCard } from '../../components/events/EventCard';
import type { PublicEventRegistration, RefundEnvelope } from '../../api/types';
import { BackToComplexButton, ExpiredState, ProcessingState } from './EventPageStates';

const t = BOOKING_MESSAGES;

function CancelledResult({ refund, slug }: { refund: RefundEnvelope; slug: string }) {
  return (
    <StatusHero
      icon={CheckCircle2}
      tone={refund.status === 'manual' ? 'warning' : 'success'}
      size="large"
      title={t.events.cancelledResultTitle}
      // The server's own Spanish sentence for this exact outcome; rendered as it comes.
      description={refund.message || t.publicBooking.cancelledSuccessfully}
      descriptionClassName="max-w-sm"
    >
      {refund.amount !== undefined && (
        <p className="text-text-primary text-base font-semibold">{formatPrice(refund.amount)}</p>
      )}
      <BackToComplexButton slug={slug} />
    </StatusHero>
  );
}

function CancelSpot({
  cancel,
  registration,
}: {
  cancel: ReturnType<typeof useCancelEventRegistration>;
  registration: PublicEventRegistration;
}) {
  const [confirmOpen, setConfirmOpen] = useState(false);
  const { within_window: withinWindow, deadline } = registration.cancellation;

  return (
    <div className="flex flex-col gap-3">
      <p className="text-text-secondary text-center text-sm">
        {withinWindow ? `${t.events.refundUntilPrefix} ${formatDeadline(deadline)}.` : t.events.refundWindowPassed}
      </p>
      <Button
        variant="destructive"
        className="min-h-12 w-full"
        onClick={() => {
          setConfirmOpen(true);
        }}
      >
        {t.events.cancelSpot}
      </Button>
      <ConfirmDialog
        open={confirmOpen}
        onClose={() => {
          setConfirmOpen(false);
        }}
        onConfirm={() => {
          cancel.mutate(undefined, {
            onSettled: () => {
              setConfirmOpen(false);
            },
          });
        }}
        title={t.events.cancelConfirmTitle}
        description={withinWindow ? t.events.cancelConfirmRefund : t.events.cancelConfirmNoRefund}
        confirmLabel={t.events.cancelConfirmYes}
        variant="destructive"
        isLoading={cancel.isPending}
      />
    </div>
  );
}

interface RegistrationViewProps {
  slug: string;
  token: string;
  eventPath: string;
  registration: PublicEventRegistration;
  /** `?status=pending`: the checkout returned while the payment is still under review. */
  underReview: boolean;
  timedOut: boolean;
  onRefresh: () => void;
}

/** The person's own spot after the checkout returns: one state per registration status. */
export function RegistrationView({
  slug,
  token,
  eventPath,
  registration,
  underReview,
  timedOut,
  onRefresh,
}: RegistrationViewProps) {
  const cancelled = useCancelEventRegistration(token);
  if (cancelled.data) {
    return <CancelledResult refund={cancelled.data.refund} slug={slug} />;
  }

  switch (registration.status) {
    case 'confirmed':
      return (
        <StatusHero
          icon={CheckCircle2}
          tone="success"
          size="large"
          title={t.events.confirmedTitle}
          description={t.events.confirmedDescription}
          descriptionClassName="max-w-sm"
        >
          <div className="flex w-full max-w-sm flex-col gap-6">
            <EventCard event={registration.event} headingLevel="h2" />
            {registration.cancellation.can_cancel && <CancelSpot cancel={cancelled} registration={registration} />}
          </div>
        </StatusHero>
      );
    case 'cancelled':
      return (
        <StatusHero
          icon={CheckCircle2}
          tone="neutral"
          title={t.events.cancelledByYouTitle}
          description={t.publicBooking.cancelledSuccessfully}
        >
          <BackToComplexButton slug={slug} />
        </StatusHero>
      );
    case 'expired':
      return <ExpiredState eventPath={eventPath} slug={slug} />;
    default:
      return <ProcessingState underReview={underReview} timedOut={timedOut} onRefresh={onRefresh} />;
  }
}
