import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { authApi } from '../api/auth.api';
import { useAuthSuccessHandler } from './authSuccess';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpStatus, getHttpErrorMessage } from '@/shared/lib/utils';
import type { GoogleNeedsProfileResponse } from '@/shared/types/api.types';

function needsProfile(data: unknown): data is GoogleNeedsProfileResponse {
  return typeof data === 'object' && data !== null && 'needs_profile' in data;
}

/**
 * Handles the `POST /auth/google` credential exchange:
 * - An existing account logs in exactly like `useLogin`'s success path
 *   (same store updates, same redirect target logic).
 * - An unknown email hands off to `/register/google` with the profile
 *   token and preview carried in router state, never the URL.
 * - Any failure maps to a toast — never a form (there is no form on this
 *   step, just the Google button).
 */
export function useGoogleSignIn() {
  const t = ES_AR;
  const navigate = useNavigate();
  const handleAuthSuccess = useAuthSuccessHandler();

  return useMutation({
    mutationFn: (credential: string) => authApi.googleSignIn(credential),
    onSuccess: (data) => {
      if (needsProfile(data)) {
        void navigate('/register/google', {
          state: { profile_token: data.profile_token, profile: data.profile },
        });
        return;
      }
      handleAuthSuccess(data);
    },
    onError: (error: unknown) => {
      const status = getHttpStatus(error);

      if (status === 429) {
        toast.error(t.auth.rateLimitError);
        return;
      }
      // The server answers 503 with a fixed English sentence
      // (`"google sign-in is not configured"`) meant for logs, not people —
      // show the Spanish copy instead of that literal text.
      if (status === 503) {
        toast.error(t.auth.googleUnavailable);
        return;
      }
      // Generic message for inactive/locked accounts, same reasoning as
      // useLogin's invalidCredentials: doesn't say which case it was. Unlike
      // useLogin, no "check your inbox" hint: Google already proved the
      // address, so an unverified account is promoted and signs in here; a
      // 401 on this path is never about verification.
      if (status === 401) {
        toast.error(t.auth.invalidCredentials);
        return;
      }

      toast.error(getHttpErrorMessage(error, t.auth.googleSignInError));
    },
  });
}
