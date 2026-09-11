import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { authApi } from '@/features/auth';
import { useStore } from '@/shared/stores';

export function useVerifyEmail() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { logout } = useStore();
  const token = searchParams.get('token');
  const [countdown, setCountdown] = useState(3);

  const query = useQuery({
    // The token is part of the key so the query cache — not a `cancelled`
    // ref — is what dedupes the call: React StrictMode mounts this hook
    // twice in development, and without that dedup the second mount fired a
    // second POST that consumed the (single-use) token again and could
    // overwrite the first request's success with "already used".
    queryKey: ['auth', 'verify-email', token],
    queryFn: async ({ signal }) => {
      await authApi.verifyEmail(token ?? '', signal);
      // Clear any existing session (e.g. opened from phone with a
      // different account) now that the new one is confirmed.
      authApi.logout().catch(() => {
        /* ignore — best-effort cleanup */
      });
      logout();
      return null;
    },
    enabled: !!token,
    // The token is single-use — retrying a failed attempt would consume it
    // again and could turn a real success into a false "already used" error.
    retry: false,
    staleTime: Infinity,
  });

  const status: 'loading' | 'success' | 'error' = !token
    ? 'error'
    : query.isError
      ? 'error'
      : query.isSuccess
        ? 'success'
        : 'loading';

  // Auto-redirect to login after successful verification.
  useEffect(() => {
    if (status !== 'success') return;
    if (countdown <= 0) {
      void navigate('/login', { replace: true });
      return;
    }
    const id = setTimeout(() => {
      setCountdown((c) => c - 1);
    }, 1000);
    return () => {
      clearTimeout(id);
    };
  }, [status, countdown, navigate]);

  return { status, countdown };
}
