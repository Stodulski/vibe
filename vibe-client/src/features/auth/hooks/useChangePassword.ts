import { useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { authApi } from '../api/auth.api';
import { useStore } from '@/shared/stores';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ChangePasswordData {
  current_password: string;
  new_password: string;
}

export function useChangePassword() {
  return useMutation({
    mutationFn: (data: ChangePasswordData) =>
      authApi.updateMe({
        current_password: data.current_password,
        new_password: data.new_password,
      }),
    onSuccess: () => {
      toast.success(t.auth.passwordChanged);
      useStore.getState().logout();
      window.location.href = '/login';
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.auth.passwordChangeError));
    },
  });
}
