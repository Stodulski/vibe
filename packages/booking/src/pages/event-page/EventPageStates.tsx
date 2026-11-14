import { AlertTriangle, CalendarX, CheckCircle2, Clock, XCircle } from 'lucide-react';
import { Button, StatusHero } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicComplexPath } from '../../lib/publicPaths';

const t = BOOKING_MESSAGES;

export function BackToComplexButton({ slug }: { slug: string }) {
  return (
    <Button asChild variant="outline" className="min-h-12">
      <a href={publicComplexPath(slug)}>{t.events.backToComplex}</a>
    </Button>
  );
}

export function EventNotFoundState({ slug }: { slug: string }) {
  return (
    <StatusHero
      icon={CalendarX}
      tone="neutral"
      title={t.events.eventNotFound}
      description={t.events.eventNotFoundDescription}
    >
      <BackToComplexButton slug={slug} />
    </StatusHero>
  );
}

export function EventLoadErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <StatusHero
      icon={AlertTriangle}
      tone="warning"
      title={t.events.eventLoadError}
      description={t.publicBooking.complexLoadErrorDescription}
    >
      <Button variant="outline" className="min-h-12" onClick={onRetry}>
        {t.publicBooking.tryAgain}
      </Button>
    </StatusHero>
  );
}

export function RegistrationNotFoundState({ slug, eventPath }: { slug: string; eventPath: string }) {
  return (
    <StatusHero
      icon={XCircle}
      tone="neutral"
      title={t.publicBooking.bookingNotFound}
      description={t.events.registrationNotFoundDescription}
    >
      <div className="flex flex-col gap-2">
        <Button asChild className="min-h-12">
          <a href={eventPath}>{t.events.signUp}</a>
        </Button>
        <BackToComplexButton slug={slug} />
      </div>
    </StatusHero>
  );
}

export function ProcessingState({
  underReview,
  timedOut,
  onRefresh,
}: {
  underReview: boolean;
  timedOut: boolean;
  onRefresh: () => void;
}) {
  return (
    <StatusHero
      icon={Clock}
      tone={underReview ? 'info' : 'neutral'}
      title={underReview ? t.events.underReviewTitle : t.events.processingTitle}
      description={underReview ? t.events.underReviewDescription : t.events.processingDescription}
    >
      {timedOut && (
        <Button variant="outline" className="min-h-12" onClick={onRefresh}>
          {t.events.refreshStatus}
        </Button>
      )}
    </StatusHero>
  );
}

export function ExpiredState({ eventPath, slug }: { eventPath: string; slug: string }) {
  return (
    <StatusHero
      icon={Clock}
      tone="warning"
      title={t.events.expiredTitle}
      description={t.events.expiredDescription}
    >
      <div className="flex flex-col gap-2">
        <Button asChild className="min-h-12">
          <a href={eventPath}>{t.events.expiredAction}</a>
        </Button>
        <BackToComplexButton slug={slug} />
      </div>
    </StatusHero>
  );
}

export function PaymentFailedState({ onRetry, isRetrying }: { onRetry: () => void; isRetrying: boolean }) {
  return (
    <StatusHero
      icon={XCircle}
      tone="error"
      title={t.publicBooking.paymentFailed}
      description={t.events.paymentFailedDescription}
    >
      <Button className="min-h-12" onClick={onRetry} disabled={isRetrying}>
        {t.events.paymentFailedAction}
      </Button>
    </StatusHero>
  );
}

export function CancelledEventNotice() {
  return (
    <StatusHero
      icon={CalendarX}
      tone="warning"
      title={t.events.cancelledTitle}
      description={t.events.cancelledDescription}
    />
  );
}

export function FullEventNotice() {
  return (
    <p role="status" className="text-text-secondary text-center text-sm">
      <CheckCircle2 className="mr-1 inline size-4 align-text-bottom" aria-hidden="true" />
      {t.events.fullDescription}
    </p>
  );
}

export function SignUpClosedNotice() {
  return (
    <p role="status" className="text-text-secondary text-center text-sm">
      <CheckCircle2 className="mr-1 inline size-4 align-text-bottom" aria-hidden="true" />
      {t.events.signUpClosed}
    </p>
  );
}
