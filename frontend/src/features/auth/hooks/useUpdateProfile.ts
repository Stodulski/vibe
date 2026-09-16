import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { authApi } from '../api/auth.api';
import { setSessionUser } from './session';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface UpdateProfileData {
  first_name: string;
  last_name: string;
  email: string;
  phone: string;
}

export function useUpdateProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateProfileData) => authApi.updateMe(data),
    onSuccess: (data) => {
      // The response already carries the updated user, so the cache is written
      // straight through rather than invalidated: the profile screen shows the
      // new name immediately instead of flickering through a refetch. This
      // runs regardless of `email_change`: even a "failed" email-change
      // request still means the rest of the PUT was saved.
      setSessionUser(queryClient, data.user, data.pending_email);

      // `email_change` says outright what happened to a requested email
      // change, so this no longer has to infer it from comparing the
      // submitted address against the returned one (which never changes
      // immediately — see `PUT /auth/me`'s contract).
      switch (data.email_change) {
        case 'requested':
          toast.success(t.auth.emailChangeConfirmationSent);
          break;
        case 'failed':
          toast.error(t.auth.emailChangeRequestFailed);
          break;
        case 'none':
          toast.success(t.auth.profileUpdated);
          break;
      }
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.auth.updateError));
    },
  });
}
