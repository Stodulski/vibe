import { useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { authApi } from '../api/auth.api';
import { useStore } from '@/shared/stores';
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
  const { setUser } = useStore();

  return useMutation({
    mutationFn: (data: UpdateProfileData) => authApi.updateMe(data),
    onSuccess: (data) => {
      // The store is the single source of truth for `user` — see useAuth.ts
      // (06-auth-shared-tooling.md M5). Writing it into the query cache too
      // used to risk the two disagreeing (e.g. after queryClient.clear()).
      setUser(data.user);
      toast.success(t.auth.profileUpdated);
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.auth.updateError));
    },
  });
}
