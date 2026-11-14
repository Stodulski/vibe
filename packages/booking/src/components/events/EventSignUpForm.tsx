import { useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { Lock } from 'lucide-react';
import { LoadingButton, formatPrice, submitHandler, useAppForm } from '@vibe/ui';
import { useEventSignUp } from '../../hooks/useEventSignUp';
import { useBookingConfig } from '../../config';
import { BOOKING_MESSAGES } from '../../messages';
import { EventSignUpFields } from './EventSignUpFields';
import { eventSignUpSchema, type EventSignUpFormData } from '../../schemas/event-signup.schema';
import type { PublicOpenPlayEvent } from '../../api/types';

const t = BOOKING_MESSAGES;

function EventPriceSummary({ event }: { event: PublicOpenPlayEvent }) {
  return (
    <dl className="text-sm">
      <div className="flex items-baseline justify-between">
        <dt className="text-text-secondary">{t.events.priceLabel}</dt>
        <dd className="text-text-primary tabular-nums">{formatPrice(event.price)}</dd>
      </div>
      <div className="flex items-baseline justify-between">
        <dt className="text-text-secondary">{t.serviceFee.label}</dt>
        <dd className="text-text-primary tabular-nums">{formatPrice(event.service_fee)}</dd>
      </div>
      <div className="flex items-baseline justify-between pt-1">
        <dt className="text-text-secondary">{t.publicBooking.totalOnline}</dt>
        <dd className="text-text-primary font-semibold tabular-nums">{formatPrice(event.total_price)}</dd>
      </div>
    </dl>
  );
}

interface EventSignUpFormProps {
  slug: string;
  event: PublicOpenPlayEvent;
}

/** Name and phone required, email optional. Submitting holds the spot and hands the person to the Mercado Pago checkout. */
export function EventSignUpForm({ slug, event }: EventSignUpFormProps) {
  const { privacyUrl } = useBookingConfig();
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
    <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4" noValidate>
      <div className="space-y-1">
        <h2 className="text-text-primary text-lg font-bold">{t.events.formTitle}</h2>
        <p className="text-text-secondary text-sm">{t.events.formDescription}</p>
      </div>

      <EventSignUpFields register={register} control={control} errors={errors} />

      <div className="space-y-2 pt-1">
        <EventPriceSummary event={event} />
        {errorMessage && (
          <p role="alert" className="text-error-text text-sm">
            {errorMessage}
          </p>
        )}
        <LoadingButton
          type="submit"
          className="shadow-brand h-14 w-full rounded-xl text-base font-semibold"
          loading={signUp.isPending}
          loadingText={t.publicBooking.generatingPaymentLink}
        >
          <Lock className="size-4 shrink-0" aria-hidden="true" />
          {t.publicBooking.paySecurely}
        </LoadingButton>
      </div>

      <p className="text-text-tertiary text-center text-xs">
        {t.events.privacyNotice}{' '}
        <a
          href={privacyUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="text-primary-400 underline underline-offset-2"
        >
          {t.publicBooking.privacyNoticeLink}
        </a>
      </p>
    </form>
  );
}
