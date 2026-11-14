import { useContext, useState } from 'react';
import { SkeletonCancelInfo } from '@vibe/ui';
import { HTTPError } from 'ky';
import { useEvent } from '../hooks/useEvent';
import { useEventRegistration } from '../hooks/useEventRegistration';
import { useQueryParams } from '../lib/navigation';
import { publicEventPath } from '../lib/publicPaths';
import { PublicBookingApiContext } from '../api/context';
import {
  clearEventRegistrationToken,
  readEventRegistrationToken,
} from '../lib/eventRegistrationStorage';
import { EventCard } from '../components/events/EventCard';
import { EventSignUpForm } from '../components/events/EventSignUpForm';
import { isOpenForSignUp } from '../components/events/eventFormat';
import {
  BackToComplexButton,
  CancelledEventNotice,
  EventLoadErrorState,
  EventNotFoundState,
  FullEventNotice,
  PaymentFailedState,
  RegistrationNotFoundState,
} from './event-page/EventPageStates';
import { RegistrationView } from './event-page/RegistrationView';

const PAGE = 'animate-fade-in mx-auto flex w-full max-w-md flex-col gap-6 px-4 py-8 sm:py-12';

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

  if (isLoading) return <SkeletonCancelInfo />;
  if (error instanceof HTTPError && error.response.status === 404) return <EventNotFoundState slug={slug} />;
  if (error || !event) {
    return (
      <EventLoadErrorState
        onRetry={() => {
          void refetch();
        }}
      />
    );
  }

  if (showFailure) return <PaymentFailedState onRetry={() => void retry()} isRetrying={pending} />;

  return (
    <>
      <EventCard event={event} headingLevel="h1" />
      {event.status === 'cancelled' && <CancelledEventNotice />}
      {event.status !== 'cancelled' && !isOpenForSignUp(event) && <FullEventNotice />}
      {isOpenForSignUp(event) && <EventSignUpForm slug={slug} event={event} />}
      <BackToComplexButton slug={slug} />
    </>
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

  return (
    <div className={PAGE}>
      {token ? (
        <RegistrationPage slug={slug} eventId={eventId} token={token} />
      ) : (
        <EventSignUpPage slug={slug} eventId={eventId} paymentFailed={paymentFailed} />
      )}
    </div>
  );
}

