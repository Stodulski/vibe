import { useState } from 'react';
import { HTTPError } from 'ky';
import { Button, FormField, Input } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { getSavedFormData } from '../../components/booking-form/savedFormData';
import { useJoinWaitlist } from '../../hooks/useJoinWaitlist';
import { getProblem } from '../../lib/ApiError';

const t = BOOKING_MESSAGES;

interface WaitlistFormProps {
  slug: string;
  /** The day to wait for, `YYYY-MM-DD`. */
  date: string;
}

interface Failure {
  /** The message under the email field, when the backend named that field. */
  emailError?: string;
  /** The message above the button, for everything else. */
  message?: string;
}

/** What a failed waitlist request means to the visitor. */
function describeFailure(error: unknown): Failure {
  if (error instanceof HTTPError && error.response.status === 422) {
    const emailError = getProblem(error)?.errors.find((entry) => entry.field === 'email')?.message;
    return emailError ? { emailError } : { message: t.publicBooking.waitlistError };
  }
  if (error instanceof HTTPError && error.response.status === 429) {
    return { message: t.publicBooking.waitlistRateLimited };
  }
  return { message: t.publicBooking.waitlistError };
}

/**
 * "Avisame si se libera un turno": the visitor's email goes on the waitlist of
 * one fully booked day. The address is prefilled from the saved identity of this
 * browser, when there is one, and trimmed before it is sent.
 */
export function WaitlistForm({ slug, date }: WaitlistFormProps) {
  const [email, setEmail] = useState(() => getSavedFormData().client_email?.trim() ?? '');
  const [confirmedEmail, setConfirmedEmail] = useState<string | null>(null);
  const waitlist = useJoinWaitlist(slug);

  function submitEmail() {
    const trimmed = email.trim();
    waitlist.mutate(
      { date, email: trimmed },
      {
        onSuccess: () => {
          setConfirmedEmail(trimmed);
        },
      },
    );
  }

  if (confirmedEmail !== null) {
    return (
      <p role="status" className="text-text-secondary text-sm">
        {`${t.publicBooking.waitlistSuccessPrefix} ${confirmedEmail} ${t.publicBooking.waitlistSuccessSuffix}`}
      </p>
    );
  }

  const failure = waitlist.isError ? describeFailure(waitlist.error) : null;

  return (
    <form
      noValidate
      onSubmit={(event) => {
        event.preventDefault();
        submitEmail();
      }}
      className="flex flex-col gap-3"
    >
      <h4 className="text-text-primary text-sm font-semibold">{t.publicBooking.waitlistTitle}</h4>
      <FormField label={t.publicBooking.waitlistEmailLabel} htmlFor="waitlist-email" error={failure?.emailError}>
        <Input
          id="waitlist-email"
          type="email"
          inputMode="email"
          autoComplete="email"
          value={email}
          onChange={(event) => {
            setEmail(event.target.value);
          }}
          className="h-12 text-base"
          aria-invalid={!!failure?.emailError}
        />
      </FormField>
      {failure?.message && (
        <p role="alert" className="text-error-text text-sm">
          {failure.message}
        </p>
      )}
      <Button type="submit" disabled={waitlist.isPending} className="self-start">
        {t.publicBooking.waitlistSubmit}
      </Button>
    </form>
  );
}
