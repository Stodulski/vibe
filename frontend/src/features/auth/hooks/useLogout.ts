import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { authApi } from '../api/auth.api';
import { useStore } from '@/shared/stores';

export function useLogout() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { logout } = useStore();

  return useMutation({
    mutationFn: () => authApi.logout(),
    onSettled: () => {
      logout();
      queryClient.clear();
      void navigate('/login', { replace: true });
    },
  });
}
