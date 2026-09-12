import { useState, useEffect } from 'react';
import { useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { authApi } from '../api/auth.api';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;
const COOLDOWN_SECONDS = 180;

export function useResendVerification(email: string | undefined) {
  const [cooldown, setCooldown] = useState(0);

  useEffect(() => {
    if (cooldown <= 0) return;
    const id = setInterval(() => {
      setCooldown((c) => c - 1);
    }, 1000);
    return () => {
      clearInterval(id);
    };
  }, [cooldown]);

  const mutation = useMutation({
    mutationFn: (targetEmail: string) => authApi.resendVerification(targetEmail),
    onSuccess: () => {
      toast.success(t.auth.resendVerificationSent);
      setCooldown(COOLDOWN_SECONDS);
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.auth.resendVerificationError));
    },
  });

  const handleResend = () => {
    if (!email || mutation.isPending || cooldown > 0) return;
    mutation.mutate(email);
  };

  return { resending: mutation.isPending, cooldown, handleResend };
}
