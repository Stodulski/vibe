import { useState } from 'react';
import type { FieldErrors, UseFormReturn } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { submitHandler, useAppForm } from '@vibe/ui';
import { useTournamentSignUp } from '../../hooks/useTournamentSignUp';
import { useChallengedSubmit } from '../../hooks/useChallengedSubmit';
import { useHoldChallenge } from '../../pages/complex-page/useHoldChallenge';
import { BOOKING_MESSAGES } from '../../messages';
import { PersonFields } from '../signup/PersonFields';
import { TurnstileField } from '../booking-form/TurnstileField';
import { FormErrorAlert } from '../booking-form/FormErrorAlert';
import { PrivacyNotice } from '../booking-form/PrivacyNotice';
import { SubmitFooter } from '../booking-form/SubmitFooter';
import { SignUpSummaryCard } from '../signup/SignUpSummaryCard';
import { formatLabel, matchFormatLabel, sportLabel, tournamentDates } from './tournamentModel';
import { pairSignUpSchema, type PairSignUpFormData } from '../../schemas/pair-signup.schema';
import type { PublicTournament, PublicTournamentCategory } from '../../api/types';

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

interface PairSignUpFormProps {
  slug: string;
  tournament: PublicTournament;
  category: PublicTournamentCategory;
}

/**
 * The payer's name and phone (email optional) and the partner's name and
 * phone. Submitting holds the spot and hands the payer to the Mercado Pago
 * checkout for the whole entry.
 */
export function PairSignUpForm({ slug, tournament, category }: PairSignUpFormProps) {
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
  const signUp = useTournamentSignUp(slug, tournament.id, category.id, setErrorMessage);

  // The booking hold's bot check: one invisible challenge, one token per sign-up.
  const { challenge, generation, onTokenChange } = useHoldChallenge();
  const submitWithChallenge = useChallengedSubmit<PairSignUpFormData>(challenge, (data, token) => {
    signUp.mutate({ data, token }, { onSettled: challenge.reset });
  });

  function onSubmit(data: PairSignUpFormData) {
    setErrorMessage(null);
    submitWithChallenge(data);
  }

  const serviceFee = category.service_fee ?? 0;
  const total = category.total_price ?? category.price + serviceFee;
  return (
    <div className="space-y-5">
      <SignUpSummaryCard
        ariaLabel={t.tournaments.summaryLabel}
        dateLabel={tournamentDates(tournament)}
        title={tournament.name}
        tags={[category.name, formatLabel(category.format), sportLabel(tournament.sport)]}
        details={[matchFormatLabel(category)]}
        priceLabel={t.tournaments.priceLabel}
        price={category.price}
        serviceFee={serviceFee}
        total={total}
        feeNote={t.tournaments.feeNote}
      />
      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4">
        <PlayersFieldsets register={register} control={control} errors={errors} categoryId={category.id} />
        <TurnstileField key={generation} onTokenChange={onTokenChange} />
        <SubmitFooter isLoading={signUp.isPending} totalOnline={total} type="submit" />
        <PrivacyNotice text={t.tournaments.privacyNotice} />
      </form>
      {errorMessage && <FormErrorAlert message={errorMessage} />}
    </div>
  );
}
