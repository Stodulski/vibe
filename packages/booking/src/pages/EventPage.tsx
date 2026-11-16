import { useContext, useState, type ReactNode } from 'react';
import { SkeletonCancelInfo } from '@vibe/ui';
import { HTTPError } from 'ky';
import { useEvent } from '../hooks/useEvent';
import { useEventRegistration } from '../hooks/useEventRegistration';
import { useQueryParams } from '../lib/navigation';
import { publicComplexPath, publicEventPath } from '../lib/publicPaths';
import { PublicBookingApiContext } from '../api/context';
import {
  clearEventRegistrationToken,
  readEventRegistrationToken,
} from '../lib/eventRegistrationStorage';
import { CheckoutLayout } from '../components/CheckoutLayout';
import { EventCard } from '../components/events/EventCard';
import { EventSignUpForm } from '../components/events/EventSignUpForm';
import { EventSignUpSummary } from '../components/events/EventSignUpSummary';
import { eventHasStarted, isOpenForSignUp } from '../components/events/eventFormat';
import { BOOKING_MESSAGES } from '../messages';
import {
  BackToComplexButton,
  CancelledEventNotice,
  EventLoadErrorState,
  EventNotFoundState,
  FullEventNotice,
  SignUpClosedNotice,
  PaymentFailedState,
  RegistrationNotFoundState,
} from './event-page/EventPageStates';
import { RegistrationView } from './event-page/RegistrationView';

const t = BOOKING_MESSAGES;

const PAGE = 'animate-fade-in mx-auto flex w-full max-w-md flex-col gap-6 px-4 py-8 sm:py-12';

/** The open-play checkout: the court was chosen on the complex page, so the progress starts at the data. */
const OPEN_PLAY_STEPS = [t.events.stepCourt, t.publicBooking.stepData, t.publicBooking.stepPay] as const;

/** A state that is not the sign-up keeps the page's own narrow column. */
function framed(children: ReactNode) {
  return <div className={PAGE}>{children}</div>;
}

/** The registration a returning person holds: read through the token in the URL. */
function RegistrationPage({ slug, eventId, token }: { slug: string; eventId: string; token: string }) {
  const [params] = useQueryParams();
  const underReview = params.get('status') === 'pending';
  const { data, isLoading, isError, timedOut, linkNotFound, refetch } = useEventRegistration(token);
  const eventPath = publicEventPath(slug, eventId);

  if (linkNotFound) return <RegistrationNotFoundState slug={slug} eventPath={eventPath} />;
  if (isLoading) return <SkeletonCancelInfo />;
  if (isError || !data) {
    return (
      <EventLoadErrorState
        onRetry={() => {
          void refetch();
        }}
      />
    );
  }
  return (
    <RegistrationView
      slug={slug}
      token={token}
      eventPath={eventPath}
      registration={data}
      underReview={underReview}
      timedOut={timedOut}
      onRefresh={() => {
        void refetch();
      }}
    />
  );
}

/** The checkout failed: the held spot is released with the stored token, then the form comes back. */
function usePaymentRetry(eventId: string, onDone: () => void) {
  const api = useContext(PublicBookingApiContext);
  const [pending, setPending] = useState(false);
  async function retry() {
    setPending(true);
    const token = readEventRegistrationToken(eventId);
    try {
      if (token && api) await api.cancelEventRegistration({ token });
    } catch {
      // Already released or never held: the form is offered either way.
    }
    clearEventRegistrationToken(eventId);
    setPending(false);
    onDone();
  }
  return { pending, retry };
}

function EventSignUpPage({ slug, eventId, paymentFailed }: { slug: string; eventId: string; paymentFailed: boolean }) {
  const [, setParams] = useQueryParams();
  const [showFailure, setShowFailure] = useState(paymentFailed);
  const { data: event, isLoading, error, refetch } = useEvent(slug, eventId);
  const { pending, retry } = usePaymentRetry(eventId, () => {
    setShowFailure(false);
    setParams(new URLSearchParams());
  });

  if (isLoading) return framed(<SkeletonCancelInfo />);
  if (error instanceof HTTPError && error.response.status === 404) return framed(<EventNotFoundState slug={slug} />);
  if (error || !event) {
    return framed(
      <EventLoadErrorState
        onRetry={() => {
          void refetch();
        }}
      />,
    );
  }

  if (showFailure) return framed(<PaymentFailedState onRetry={() => void retry()} isRetrying={pending} />);

  const started = eventHasStarted(event);
  const cancelled = event.status === 'cancelled';
  const open = isOpenForSignUp(event) && !started;
  if (open) {
    // The way back goes to the complex with its canchas abiertas already picked. No "Ver el complejo": this page is the checkout.
    return (
      <CheckoutLayout
        currentStep={2}
        steps={OPEN_PLAY_STEPS}
        back={{ label: t.events.backToOffers, href: `${publicComplexPath(slug)}?offer=events` }}
        summary={<EventSignUpSummary event={event} />}
      >
        <EventSignUpForm slug={slug} event={event} />
      </CheckoutLayout>
    );
  }
  return framed(
    <>
      <EventCard event={event} headingLevel="h1" />
      {cancelled && <CancelledEventNotice />}
      {!cancelled && started && <SignUpClosedNotice />}
      {!cancelled && !started && <FullEventNotice />}
      <BackToComplexButton slug={slug} />
    </>,
  );
}

/**
 * `/c/<slug>/events/<id>`: the event, its sign-up form and, when the checkout
 * returns here (`?token=...&registered=1`, `&status=pending`, or
 * `?error=payment_failed`), what became of the payment.
 */
export default function EventPage({ slug, eventId }: { slug: string; eventId: string }) {
  const [params] = useQueryParams();
  const token = params.get('token');
  const paymentFailed = params.get('error') === 'payment_failed';

  if (token) {
    return (
      <div className={PAGE}>
        <RegistrationPage slug={slug} eventId={eventId} token={token} />
      </div>
    );
  }
  return <EventSignUpPage slug={slug} eventId={eventId} paymentFailed={paymentFailed} />;
}
