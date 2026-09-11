import { useEffect, useRef, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { toast } from 'sonner';
import { authApi, type ResetPasswordDto } from '@/features/auth';
import { useStore } from '@/shared/stores';
import { getHttpStatus } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function useResetPassword() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { logout } = useStore();
  const token = searchParams.get('token');
  const [countdown, setCountdown] = useState(3);
  const clearedSession = useRef(false);

  // Clear any existing session so the user lands clean on this page. Guarded
  // by a ref (rather than relying on the effect's own cleanup) so React
  // StrictMode's mount→unmount→mount in development doesn't fire this
  // best-effort cleanup twice.
  useEffect(() => {
    if (clearedSession.current) return;
    clearedSession.current = true;
    authApi.logout().catch(() => {
      /* ignore — best-effort cleanup */
    });
    logout();
  }, [logout]);

  const mutation = useMutation({
    mutationFn: (password: string) => authApi.resetPassword(token ?? '', password),
    onError: (error: unknown) => {
      if (getHttpStatus(error) === 429) {
        toast.error(t.auth.rateLimitError);
      }
    },
  });

  const status: 'form' | 'success' | 'error' = !token
    ? 'error'
    : mutation.isSuccess
      ? 'success'
      : mutation.isError && getHttpStatus(mutation.error) !== 429
        ? 'error'
        : 'form';

  // Auto-redirect countdown after success (with proper cleanup).
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

  const onSubmit = (data: ResetPasswordDto) => {
    if (!token) return;
    mutation.mutate(data.password);
  };

  return { status, loading: mutation.isPending, countdown, onSubmit };
}
