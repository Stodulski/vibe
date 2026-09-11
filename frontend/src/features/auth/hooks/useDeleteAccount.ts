import { useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { authApi } from '../api/auth.api';
import { useStore } from '@/shared/stores';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function useDeleteAccount() {
  return useMutation({
    mutationFn: () => authApi.deleteAccount(),
    onSuccess: () => {
      toast.success(t.profile.accountDeleted);
      useStore.getState().logout();
      window.location.href = '/login';
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.profile.deleteError));
    },
  });
}
