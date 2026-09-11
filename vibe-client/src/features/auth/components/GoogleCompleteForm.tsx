import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { HTTPError } from 'ky';
import { googleCompleteSchema, type GoogleCompleteDto } from '../schemas/auth.schemas';
import { useGoogleComplete } from '../hooks/useGoogleComplete';
import { GoogleCompleteEmailField } from './google-complete/GoogleCompleteEmailField';
import { GoogleCompleteNameFields } from './google-complete/GoogleCompleteNameFields';
import { GoogleCompletePhoneField } from './google-complete/GoogleCompletePhoneField';
import { GoogleCompleteSubmit } from './google-complete/GoogleCompleteSubmit';
import { submitHandler } from '@/shared/lib/form';
import { getFieldErrors } from '@/shared/lib/serverErrors';
import type { GoogleProfilePreview } from '@/shared/types/api.types';

/** ky consumes the body before throwing, so the payload lives on `.data`. */
function errorBody(error: unknown): unknown {
  return error instanceof HTTPError ? error.data : null;
}

const SERVER_FIELDS = new Set<keyof GoogleCompleteDto>(['first_name', 'last_name', 'phone']);

/** Server-side field errors land ON their field, not only in a toast — same reasoning as `useComplexForm`. */
function applyServerFieldErrors(error: unknown, setError: ReturnType<typeof useForm<GoogleCompleteDto>>['setError']) {
  for (const [field, message] of Object.entries(getFieldErrors(errorBody(error)))) {
    if (SERVER_FIELDS.has(field as keyof GoogleCompleteDto)) {
      setError(field as keyof GoogleCompleteDto, { type: 'server', message });
    }
  }
}

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
  const googleComplete = useGoogleComplete();

  const {
    register,
    handleSubmit,
    control,
    setError,
    formState: { errors },
  } = useForm<GoogleCompleteDto>({
    resolver: zodResolver(googleCompleteSchema),
    defaultValues: {
      first_name: profile.first_name,
      last_name: profile.last_name,
      phone: '',
    },
  });

  const onSubmit = (data: GoogleCompleteDto) => {
    googleComplete.mutate(
      { profile_token: profileToken, ...data },
      {
        onError: (error) => {
          applyServerFieldErrors(error, setError);
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
