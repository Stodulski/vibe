import { useMemo, useState } from 'react';
import type { FieldErrors, UseFormReturn } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { submitHandler, useAppForm } from '@vibe/ui';
import { useTournamentSignUp } from '../../hooks/useTournamentSignUp';
import { useChallengedSubmit } from '../../hooks/useChallengedSubmit';
import { useHoldChallenge } from '../../pages/complex-page/useHoldChallenge';
import { BOOKING_MESSAGES } from '../../messages';
import { PersonFields } from '../signup/PersonFields';
import { PlayerDeclarationFields } from '../signup/PlayerDeclarationFields';
import { TurnstileField } from '../booking-form/TurnstileField';
import { FormErrorAlert } from '../booking-form/FormErrorAlert';
import { PrivacyNotice } from '../booking-form/PrivacyNotice';
import { SubmitFooter } from '../booking-form/SubmitFooter';
import { SignUpSummaryCard } from '../signup/SignUpSummaryCard';
import { formatLabel, matchFormatLabel, sportLabel, tournamentDates } from './tournamentModel';
import { categoryWhen } from './agendaModel';
import { pairRequirement } from '../../lib/declaredPlayers';
import { categoryTitle, isModality, tournamentLabel } from '../../lib/traitLabels';
import { pairSignUpSchema, type PairSignUpFormData } from '../../schemas/pair-signup.schema';
import type { PublicTournament, PublicTournamentCategory } from '../../api/types';

const t = BOOKING_MESSAGES;

type FormApi = Pick<UseFormReturn<PairSignUpFormData>, 'register' | 'control'> & {
  errors: FieldErrors<PairSignUpFormData>;
};

function PlayersFieldsets({
  register,
  control,
  errors,
  categoryId,
  declares,
}: FormApi & { categoryId: string; declares: boolean }) {
  return (
    <>
      <fieldset className="space-y-3">
        <legend className="text-text-primary mb-3 text-base font-bold">{t.tournaments.payerTitle}</legend>
        <PersonFields register={register} control={control} errors={errors} idPrefix={`pair_${categoryId}_`} withEmail />
        {declares && <PlayerDeclarationFields register={register} errors={errors} idPrefix={`pair_${categoryId}_`} />}
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
        {declares && (
          <PlayerDeclarationFields
            register={register}
            errors={errors}
            namePrefix="partner."
            idPrefix={`pair_${categoryId}_partner_`}
          />
        )}
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
 * The ticket of what the pair pays for: the category's day when it has one,
 * its title under the tournament's label, its format and its price.
 */
function PairSummary({
  tournament,
  category,
}: {
  tournament: PublicTournament;
  category: PublicTournamentCategory;
}) {
  const dateLabel = category.starts_on
    ? categoryWhen({ date: category.starts_on, time: category.starts_at })
    : tournamentDates(tournament);
  return (
    <SignUpSummaryCard
      ariaLabel={t.tournaments.summaryLabel}
      dateLabel={dateLabel}
      title={tournamentLabel(tournament.sport)}
      tags={[categoryTitle(category), formatLabel(category.format), sportLabel(tournament.sport)]}
      details={[matchFormatLabel(category)]}
      priceLabel={t.tournaments.priceLabel}
      price={category.price}
      serviceFee={category.service_fee ?? 0}
      total={category.total_price ?? category.price + (category.service_fee ?? 0)}
      feeNote={t.tournaments.feeNote}
    />
  );
}

/**
 * The payer's name and phone (email optional) and the partner's name and
 * phone. Submitting holds the spot and hands the payer to the Mercado Pago
 * checkout for the whole entry.
 */
export function PairSignUpForm({ slug, tournament, category }: PairSignUpFormProps) {
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const declares = isModality(category.modality);
  const schema = useMemo(() => pairSignUpSchema(category), [category]);
  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useAppForm<PairSignUpFormData>({
    resolver: zodResolver(schema),
    defaultValues: {
      first_name: '',
      last_name: '',
      phone: '',
      email: '',
      gender: '',
      level: '',
      partner: { first_name: '', last_name: '', phone: '', gender: '', level: '' },
    },
  });
  const signUp = useTournamentSignUp(slug, tournament.id, category, setErrorMessage);
  const requirement = pairRequirement(category);

  // The booking hold's bot check: one invisible challenge, one token per sign-up.
  const { challenge, generation, onTokenChange } = useHoldChallenge();
  const submitWithChallenge = useChallengedSubmit<PairSignUpFormData>(challenge, (data, token) => {
    signUp.mutate({ data, token }, { onSettled: challenge.reset });
  });

  function onSubmit(data: PairSignUpFormData) {
    setErrorMessage(null);
    submitWithChallenge(data);
  }

  const total = category.total_price ?? category.price + (category.service_fee ?? 0);
  return (
    <div className="space-y-5">
      <PairSummary tournament={tournament} category={category} />
      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4">
        {requirement && <p className="text-text-secondary text-sm">{requirement}</p>}
        <PlayersFieldsets
          register={register}
          control={control}
          errors={errors}
          categoryId={category.id}
          declares={declares}
        />
        <TurnstileField key={generation} onTokenChange={onTokenChange} />
        <SubmitFooter isLoading={signUp.isPending} totalOnline={total} type="submit" />
        <PrivacyNotice text={t.tournaments.privacyNotice} />
      </form>
      {errorMessage && <FormErrorAlert message={errorMessage} />}
    </div>
  );
}
