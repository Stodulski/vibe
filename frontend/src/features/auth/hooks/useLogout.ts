import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { authApi } from '../api/auth.api';
import { setSessionUser } from './session';
import { useStore } from '@/shared/stores';

export function useLogout() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const logout = useStore((s) => s.logout);

  return useMutation({
    mutationFn: () => authApi.logout(),
    onSettled: () => {
      logout();
      queryClient.clear();
      // `clear()` empties the session query too, and the `useAuth` mounted on
      // /login would immediately go ask `GET /auth/me` who is signed in — a
      // request whose answer we already know. Seeding `null` answers it, and
      // keeps the login screen from flashing its loading spinner on the way in.
      setSessionUser(queryClient, null);
      void navigate('/login', { replace: true });
    },
  });
}
