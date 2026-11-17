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
import { pairDeclaresLevel, pairRequirement } from '../../lib/declaredPlayers';
import { isModality } from '../../lib/traitLabels';
import { pairSignUpSchema, type PairSignUpFormData } from '../../schemas/pair-signup.schema';
import type { PublicTournament, PublicTournamentCategory } from '../../api/types';

const t = BOOKING_MESSAGES;

type FormApi = Pick<UseFormReturn<PairSignUpFormData>, 'register' | 'control'> & {
  errors: FieldErrors<PairSignUpFormData>;
};

interface PlayersFieldsetsProps extends FormApi {
  categoryId: string;
  /** The tournament's sport: it decides the words of the gender. */
  sport: string;
  /** The category has a modality, so each player says their gender. */
  asksGender: boolean;
  /** The category has a level or a Suma, so each player says their level. */
  asksLevel: boolean;
}

function PlayersFieldsets({ register, control, errors, categoryId, sport, asksGender, asksLevel }: PlayersFieldsetsProps) {
  const asksAnything = asksGender || asksLevel;
  return (
    <>
      <fieldset className="space-y-3">
        <legend className="text-text-primary mb-3 text-base font-bold">{t.tournaments.payerTitle}</legend>
        <PersonFields register={register} control={control} errors={errors} idPrefix={`pair_${categoryId}_`} withEmail />
        {asksAnything && (
          <PlayerDeclarationFields
            register={register}
            control={control}
            errors={errors}
            idPrefix={`pair_${categoryId}_`}
            sport={sport}
            asksGender={asksGender}
            asksLevel={asksLevel}
          />
        )}
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
        {asksAnything && (
          <PlayerDeclarationFields
            register={register}
            control={control}
            errors={errors}
            namePrefix="partner."
            idPrefix={`pair_${categoryId}_partner_`}
            sport={sport}
            asksGender={asksGender}
            asksLevel={asksLevel}
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
 * The payer's name, phone and email, and the partner's name and phone.
 * Submitting holds the spot and hands the payer to the Mercado Pago
 * checkout for the whole entry. The ticket of what the pair pays for is set
 * above it by the page (see PairSummary).
 */
export function PairSignUpForm({ slug, tournament, category }: PairSignUpFormProps) {
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  // The category with the sport it is played in: the words of its rules come from it.
  const pair = useMemo(() => ({ ...category, sport: tournament.sport }), [category, tournament.sport]);
  const askGender = isModality(category.modality);
  const askLevel = pairDeclaresLevel(pair);
  const schema = useMemo(() => pairSignUpSchema(pair), [pair]);
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
  const signUp = useTournamentSignUp(slug, tournament.id, pair, setErrorMessage);
  const requirement = pairRequirement(pair);

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
      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4">
        {requirement && <p className="text-text-secondary text-sm">{requirement}</p>}
        <PlayersFieldsets
          register={register}
          control={control}
          errors={errors}
          categoryId={category.id}
          sport={tournament.sport}
          asksGender={askGender}
          asksLevel={askLevel}
        />
        <TurnstileField key={generation} onTokenChange={onTokenChange} />
        <SubmitFooter isLoading={signUp.isPending} totalOnline={total} type="submit" />
        <PrivacyNotice text={t.tournaments.privacyNotice} />
      </form>
      {errorMessage && <FormErrorAlert message={errorMessage} />}
    </div>
  );
}
