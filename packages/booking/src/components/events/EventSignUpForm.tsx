import { useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { submitHandler, toDisplayDate, useAppForm } from '@vibe/ui';
import { format } from 'date-fns/format';
import { es } from 'date-fns/locale/es';
import { useEventSignUp } from '../../hooks/useEventSignUp';
import { keepTogether } from '../../lib/keepTogether';
import { BOOKING_MESSAGES } from '../../messages';
import { EventSignUpFields } from './EventSignUpFields';
import { eventTimeRange, endsNextDay, sportLabel } from './eventFormat';
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
  const courts = event.court_names.length > 0 ? [`${t.events.courtsLabel}: ${event.court_names.map(keepTogether).join(', ')}`] : [];
  return endsNextDay(event) ? [...courts, t.events.nextDayNote] : courts;
}

/**
 * The same checkout as a booking: the ticket of what is being paid for, then
 * name and phone required and email optional, the total beside the pay button
 * and the privacy line. Submitting holds the spot and hands the person to the
 * Mercado Pago checkout.
 */
export function EventSignUpForm({ slug, event }: EventSignUpFormProps) {
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useAppForm<EventSignUpFormData>({
    resolver: zodResolver(eventSignUpSchema),
    defaultValues: { first_name: '', last_name: '', phone: '', email: '' },
  });

  const signUp = useEventSignUp(slug, event.id, setErrorMessage);

  function onSubmit(data: EventSignUpFormData) {
    setErrorMessage(null);
    signUp.mutate(data);
  }

  return (
    <div className="space-y-5">
      <SignUpSummaryCard
        ariaLabel={t.events.summaryLabel}
        venue={event.complex_name}
        dateLabel={format(toDisplayDate(event.date), "EEEE d 'de' MMMM", { locale: es })}
        timeLabel={eventTimeRange(event)}
        title={event.name}
        tags={[sportLabel(event.sport)]}
        details={eventDetails(event)}
        priceLabel={t.events.priceLabel}
        price={event.price}
        serviceFee={event.service_fee}
        total={event.total_price}
        feeNote={t.events.feeNote}
        cancellationHours={event.cancellation_hours}
      />

      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4">
        <EventSignUpFields register={register} control={control} errors={errors} />
        <SubmitFooter isLoading={signUp.isPending} totalOnline={event.total_price} type="submit" />
        <PrivacyNotice text={t.events.privacyNotice} />
      </form>

      {errorMessage && <FormErrorAlert message={errorMessage} />}
    </div>
  );
}
