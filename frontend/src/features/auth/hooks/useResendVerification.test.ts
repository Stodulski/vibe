import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { toast } from 'sonner';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { createQueryWrapper } from '@/test/test-utils';

vi.mock('@/features/auth/api/auth.api', () => ({
  authApi: { resendVerification: vi.fn() },
}));

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

describe('useResendVerification', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('starts the cooldown after a successful resend', async () => {
    const { authApi } = await import('@/features/auth/api/auth.api');
    vi.mocked(authApi.resendVerification).mockResolvedValueOnce({ message: 'ok' });

    const { useResendVerification } = await import('./useResendVerification');
    const { result } = renderHook(() => useResendVerification('juan@test.com'), {
      wrapper: createQueryWrapper(),
    });

    act(() => {
      result.current.handleResend();
    });

    await waitFor(() => {
      expect(result.current.cooldown).toBeGreaterThan(0);
    });
    expect(toast.success).toHaveBeenCalledWith(ES_AR.auth.resendVerificationSent);
  });

  it('does not send a second request while one is already pending', async () => {
    const { authApi } = await import('@/features/auth/api/auth.api');
    vi.mocked(authApi.resendVerification).mockReturnValueOnce(
      new Promise(() => {
        /* never resolves */
      }),
    );

    const { useResendVerification } = await import('./useResendVerification');
    const { result } = renderHook(() => useResendVerification('juan@test.com'), {
      wrapper: createQueryWrapper(),
    });

    act(() => {
      result.current.handleResend();
    });
    await waitFor(() => {
      expect(result.current.resending).toBe(true);
    });
    act(() => {
      result.current.handleResend();
    });

    expect(authApi.resendVerification).toHaveBeenCalledTimes(1);
  });

  // The old catch block always showed the generic fallback message,
  // regardless of what the backend actually said. `getHttpErrorMessage`
  // surfaces the server's own reason (e.g. "already verified") when one is
  // available, only falling back to the generic copy otherwise.
  it('surfaces the backend error message on resend failure instead of only the generic fallback', async () => {
    const { authApi } = await import('@/features/auth/api/auth.api');
    const backendError = await makeConsumedHttpError(400, { title: 'Bad Request', detail: 'Email ya verificado' });
    vi.mocked(authApi.resendVerification).mockRejectedValueOnce(backendError);

    const { useResendVerification } = await import('./useResendVerification');
    const { result } = renderHook(() => useResendVerification('juan@test.com'), {
      wrapper: createQueryWrapper(),
    });

    act(() => {
      result.current.handleResend();
    });

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Email ya verificado');
    });
    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.auth.resendVerificationError);
  });
});
