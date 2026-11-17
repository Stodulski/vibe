import { useMemo, useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { submitHandler, useAppForm } from '@vibe/ui';
import { useEventSignUp } from '../../hooks/useEventSignUp';
import { useChallengedSubmit } from '../../hooks/useChallengedSubmit';
import { useHoldChallenge } from '../../pages/complex-page/useHoldChallenge';
import { openPlayRequirement } from '../../lib/declaredPlayers';
import { BOOKING_MESSAGES } from '../../messages';
import { EventSignUpFields } from './EventSignUpFields';
import { TurnstileField } from '../booking-form/TurnstileField';
import { FormErrorAlert } from '../booking-form/FormErrorAlert';
import { PrivacyNotice } from '../booking-form/PrivacyNotice';
import { SubmitFooter } from '../booking-form/SubmitFooter';
import { eventSignUpSchema, type EventSignUpFormData } from '../../schemas/event-signup.schema';
import type { PublicOpenPlayEvent } from '../../api/types';

const t = BOOKING_MESSAGES;

interface EventSignUpFormProps {
  slug: string;
  event: PublicOpenPlayEvent;
}

/**
 * The fields of an open play's checkout: name, phone and email required, the
 * total beside the pay button and the privacy line. The ticket of
 * what is paid for is set above it by the page (see EventSignUpSummary).
 * Submitting holds the spot and hands the person to the Mercado Pago checkout.
 */
export function EventSignUpForm({ slug, event }: EventSignUpFormProps) {
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const schema = useMemo(() => eventSignUpSchema(event), [event]);

  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useAppForm<EventSignUpFormData>({
    resolver: zodResolver(schema),
    defaultValues: { first_name: '', last_name: '', phone: '', email: '', gender: '', level: '' },
  });

  const signUp = useEventSignUp(slug, event, setErrorMessage);
  const requirement = openPlayRequirement(event);

  // The booking hold's bot check: one invisible challenge, one token per sign-up.
  const { challenge, generation, onTokenChange } = useHoldChallenge();
  const submitWithChallenge = useChallengedSubmit<EventSignUpFormData>(challenge, (data, token) => {
    signUp.mutate({ data, token }, { onSettled: challenge.reset });
  });

  function onSubmit(data: EventSignUpFormData) {
    setErrorMessage(null);
    submitWithChallenge(data);
  }

  return (
    <div className="space-y-5">
      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4">
        {requirement && <p className="text-text-secondary text-sm">{requirement}</p>}
        <EventSignUpFields register={register} control={control} errors={errors} event={event} />
        <TurnstileField key={generation} onTokenChange={onTokenChange} />
        <SubmitFooter isLoading={signUp.isPending} totalOnline={event.total_price} type="submit" />
        <PrivacyNotice text={t.events.privacyNotice} />
      </form>

      {errorMessage && <FormErrorAlert message={errorMessage} />}
    </div>
  );
}
