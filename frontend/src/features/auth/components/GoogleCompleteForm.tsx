import { zodResolver } from '@hookform/resolvers/zod';
import { googleCompleteSchema, type GoogleCompleteDto } from '../schemas/auth.schema';
import { useGoogleComplete } from '../hooks/useGoogleComplete';
import { useAbandonedGoogleSignupLead } from './google-complete/useAbandonedGoogleSignupLead';
import { GoogleCompleteEmailField } from './google-complete/GoogleCompleteEmailField';
import { GoogleCompleteNameFields } from './google-complete/GoogleCompleteNameFields';
import { GoogleCompletePhoneField } from './google-complete/GoogleCompletePhoneField';
import { GoogleCompleteSubmit } from './google-complete/GoogleCompleteSubmit';
import { useAppForm, submitHandler } from '@/shared/lib/form';
import { applyServerFieldErrors } from '@/shared/lib/serverFieldErrors';
import type { GoogleProfilePreview } from '@/shared/types/api.types';

/**
 * The fields this step owns. Passed explicitly rather than left to the
 * helper's `getValues()` default: `email` is rendered read-only from the
 * Google profile and is not a registered input, so an `email` error from the
 * server belongs on `root`, not under a field nobody can edit.
 */
const SERVER_FIELDS: readonly string[] = ['first_name', 'last_name', 'phone'];

interface GoogleCompleteFormProps {
  profileToken: string;
  profile: GoogleProfilePreview;
}

/**
 * Second step of Google sign-up: the account is real (`profile_token` is
 * short-lived and single-use) but has no phone number yet. Prefilled from
 * the Google profile, editable, phone reuses the register form's
 * validation and input.
 */
export function GoogleCompleteForm({ profileToken, profile }: GoogleCompleteFormProps) {
  const form = useAppForm<GoogleCompleteDto>({
    resolver: zodResolver(googleCompleteSchema),
    defaultValues: {
      first_name: profile.first_name,
      last_name: profile.last_name,
      phone: '',
    },
  });
  const {
    register,
    handleSubmit,
    control,
    getValues,
    formState: { errors },
  } = form;

  const lead = useAbandonedGoogleSignupLead(profile.email, getValues);
  const googleComplete = useGoogleComplete({ onAccountCreated: lead.markAccountCreated });

  const onSubmit = (data: GoogleCompleteDto) => {
    googleComplete.mutate(
      { profile_token: profileToken, ...data },
      {
        onError: (error) => {
          applyServerFieldErrors(form, error, { fields: SERVER_FIELDS });
        },
      },
    );
  };

  return (
    <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4" noValidate>
      <GoogleCompleteEmailField email={profile.email} />
      <GoogleCompleteNameFields register={register} errors={errors} />
      <GoogleCompletePhoneField control={control} errors={errors} />
      <GoogleCompleteSubmit isPending={googleComplete.isPending} />
    </form>
  );
}
