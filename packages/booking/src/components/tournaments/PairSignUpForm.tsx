import { useState } from 'react';
import type { FieldErrors, UseFormReturn } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Lock } from 'lucide-react';
import { LoadingButton, submitHandler, useAppForm } from '@vibe/ui';
import { useTournamentSignUp } from '../../hooks/useTournamentSignUp';
import { useBookingConfig } from '../../config';
import { BOOKING_MESSAGES } from '../../messages';
import { PersonFields } from '../signup/PersonFields';
import { PriceSummary } from '../signup/PriceSummary';
import { pairSignUpSchema, type PairSignUpFormData } from '../../schemas/pair-signup.schema';
import type { PublicTournamentCategory } from '../../api/types';

const t = BOOKING_MESSAGES;

type FormApi = Pick<UseFormReturn<PairSignUpFormData>, 'register' | 'control'> & {
  errors: FieldErrors<PairSignUpFormData>;
};

function PlayersFieldsets({ register, control, errors, categoryId }: FormApi & { categoryId: string }) {
  return (
    <>
      <fieldset className="space-y-3">
        <legend className="text-text-primary mb-3 text-base font-bold">{t.tournaments.payerTitle}</legend>
        <PersonFields register={register} control={control} errors={errors} idPrefix={`pair_${categoryId}_`} withEmail />
      </fieldset>

      <fieldset className="space-y-3">
        <legend className="text-text-primary mb-3 text-base font-bold">{t.tournaments.partnerTitle}</legend>
        <PersonFields
          register={register}
          control={control}
          errors={errors}
          namePrefix="partner."
          idPrefix={`pair_${categoryId}_partner_`}
          labels={{
            firstName: t.tournaments.partnerFirstName,
            lastName: t.tournaments.partnerLastName,
            phone: t.tournaments.partnerPhone,
            phoneHelp: t.tournaments.partnerPhoneHelper,
          }}
        />
      </fieldset>
    </>
  );
}

function PrivacyNotice() {
  const { privacyUrl } = useBookingConfig();
  return (
    <p className="text-text-tertiary text-center text-xs">
      {t.tournaments.privacyNotice}{' '}
      <a
        href={privacyUrl}
        target="_blank"
        rel="noopener noreferrer"
        className="text-primary-400 whitespace-nowrap underline underline-offset-2"
      >
        {t.publicBooking.privacyNoticeLink}
      </a>
    </p>
  );
}

interface PairSignUpFormProps {
  slug: string;
  tournamentId: string;
  category: PublicTournamentCategory;
}

/**
 * The payer's name and phone (email optional) and the partner's name and
 * phone. Submitting holds the spot and hands the payer to the Mercado Pago
 * checkout for the whole entry.
 */
export function PairSignUpForm({ slug, tournamentId, category }: PairSignUpFormProps) {
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useAppForm<PairSignUpFormData>({
    resolver: zodResolver(pairSignUpSchema),
    defaultValues: { first_name: '', last_name: '', phone: '', email: '', partner: { first_name: '', last_name: '', phone: '' } },
  });
  const signUp = useTournamentSignUp(slug, tournamentId, category.id, setErrorMessage);

  function onSubmit(data: PairSignUpFormData) {
    setErrorMessage(null);
    signUp.mutate(data);
  }

  const serviceFee = category.service_fee ?? 0;
  return (
    <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-5" noValidate>
      <p className="text-text-secondary text-sm">{t.tournaments.formDescription}</p>
      <PlayersFieldsets register={register} control={control} errors={errors} categoryId={category.id} />
      <div className="space-y-4 pt-2">
        <PriceSummary
          label={t.tournaments.priceLabel}
          price={category.price}
          serviceFee={serviceFee}
          total={category.total_price ?? category.price + serviceFee}
        />
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
      <PrivacyNotice />
    </form>
  );
}
