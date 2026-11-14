import { Ban, CheckCircle2 } from 'lucide-react';
import { StatusHero } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { useCancelEventRegistration } from '../../hooks/useEventRegistration';
import { EventCard } from '../../components/events/EventCard';
import { CancelRegistration, CancelledResult, type CancelCopy } from '../../components/registration/CancelRegistration';
import type { PublicEventRegistration } from '../../api/types';
import { BackToComplexButton, ExpiredState, ProcessingState } from './EventPageStates';

const t = BOOKING_MESSAGES;

const CANCEL_COPY: CancelCopy = {
  action: t.events.cancelSpot,
  confirmTitle: t.events.cancelConfirmTitle,
  confirmRefund: t.events.cancelConfirmRefund,
  confirmNoRefund: t.events.cancelConfirmNoRefund,
};

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
    return <CancelledResult refund={cancelled.data.refund} slug={slug} title={t.events.cancelledResultTitle} />;
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
          <div className="-mx-2 flex w-[calc(100%+1rem)] flex-col gap-6 sm:mx-0 sm:w-full">
            <EventCard
              event={registration.event}
              headingLevel="h2"
              paid={registration.price + registration.service_fee}
            />
            {registration.cancellation.can_cancel && (
              <CancelRegistration
                withinWindow={registration.cancellation.within_window}
                deadline={registration.cancellation.deadline}
                cancel={cancelled}
                copy={CANCEL_COPY}
              />
            )}
          </div>
        </StatusHero>
      );
    case 'cancelled':
      return (
        <StatusHero
          icon={Ban}
          tone="neutral"
          size="large"
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
