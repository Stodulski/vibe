import { useMemo, useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { submitHandler, toDisplayDate, useAppForm } from '@vibe/ui';
import { format } from 'date-fns/format';
import { es } from 'date-fns/locale/es';
import { useEventSignUp } from '../../hooks/useEventSignUp';
import { useChallengedSubmit } from '../../hooks/useChallengedSubmit';
import { useHoldChallenge } from '../../pages/complex-page/useHoldChallenge';
import { courtFactsLine } from '../../lib/courtFacts';
import { keepTogether } from '../../lib/keepTogether';
import { openPlayRequirement } from '../../lib/declaredPlayers';
import { openPlayTitle, isModality } from '../../lib/traitLabels';
import { BOOKING_MESSAGES } from '../../messages';
import { EventSignUpFields } from './EventSignUpFields';
import { eventTimeRange, endsNextDay, sportLabel } from './eventFormat';
import { TurnstileField } from '../booking-form/TurnstileField';
import { FormErrorAlert } from '../booking-form/FormErrorAlert';
import { PrivacyNotice } from '../booking-form/PrivacyNotice';
import { SubmitFooter } from '../booking-form/SubmitFooter';
import { SignUpSummaryCard } from '../signup/SignUpSummaryCard';
import { eventSignUpSchema, type EventSignUpFormData } from '../../schemas/event-signup.schema';
import type { PublicOpenPlayEvent } from '../../api/types';

const t = BOOKING_MESSAGES;

interface EventSignUpFormProps {
  slug: string;
  event: PublicOpenPlayEvent;
}

function eventDetails(event: PublicOpenPlayEvent): string[] {
  return endsNextDay(event) ? [t.events.nextDayNote] : [];
}

/** One row per court: its name kept together, then roof, surface and lighting the way the booking flow words them. */
function eventCourts(event: PublicOpenPlayEvent) {
  return event.courts.map((court) => ({ id: court.id, name: keepTogether(court.name), facts: courtFactsLine(court) }));
}

/**
 * The same checkout as a booking: the ticket of what is being paid for, then
 * name and phone required and email optional, the total beside the pay button
 * and the privacy line. Submitting holds the spot and hands the person to the
 * Mercado Pago checkout.
 */
export function EventSignUpForm({ slug, event }: EventSignUpFormProps) {
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const declares = isModality(event.modality);
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
      <SignUpSummaryCard
        ariaLabel={t.events.summaryLabel}
        venue={event.complex_name}
        dateLabel={format(toDisplayDate(event.date), "EEEE d 'de' MMMM", { locale: es })}
        timeLabel={eventTimeRange(event)}
        title={openPlayTitle(event)}
        tags={[sportLabel(event.sport)]}
        details={eventDetails(event)}
        courts={eventCourts(event)}
        courtsLabel={t.events.courtsLabel}
        priceLabel={t.events.priceLabel}
        price={event.price}
        serviceFee={event.service_fee}
        total={event.total_price}
        feeNote={t.events.feeNote}
        cancellationHours={event.cancellation_hours}
      />

      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4">
        {requirement && <p className="text-text-secondary text-sm">{requirement}</p>}
        <EventSignUpFields register={register} control={control} errors={errors} declares={declares} />
        <TurnstileField key={generation} onTokenChange={onTokenChange} />
        <SubmitFooter isLoading={signUp.isPending} totalOnline={event.total_price} type="submit" />
        <PrivacyNotice text={t.events.privacyNotice} />
      </form>

      {errorMessage && <FormErrorAlert message={errorMessage} />}
    </div>
  );
}
