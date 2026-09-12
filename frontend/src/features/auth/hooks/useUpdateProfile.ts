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
      // new name immediately instead of flickering through a refetch.
      setSessionUser(queryClient, data.user);
      toast.success(t.auth.profileUpdated);
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.auth.updateError));
    },
  });
}
