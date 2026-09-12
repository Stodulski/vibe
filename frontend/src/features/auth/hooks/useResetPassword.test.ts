import { renderHook, waitFor, act } from '@testing-library/react';
import { toast } from 'sonner';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeConsumedHttpError } from '@/test/factories';
import { createWrapper } from '@/test/test-utils';

vi.mock('@/features/auth/api/auth.api', () => ({
  authApi: {
    resetPassword: vi.fn(),
    logout: vi.fn().mockResolvedValue({}),
  },
}));

vi.mock('@/shared/stores', () => ({
  useStore: () => ({ logout: vi.fn() }),
}));

vi.mock('sonner', () => ({
  toast: { error: vi.fn() },
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe('useResetPassword', () => {
  // The old catch block cast the rejection to `{ response?: { status?: number } }`
  // and read `.response.status` straight off it. That crashes when the
  // rejection isn't an object at all (e.g. a dropped connection, or a
  // TimeoutError with no `.response`), which threw *inside* the catch and
  // left `setStatus('error')` never called — the countdown state got stuck
  // showing the form forever, with no error and no way to retry.
  it('resolves to the error state instead of crashing when the rejection has no response shape', async () => {
    const { authApi } = await import('@/features/auth/api/auth.api');
    vi.mocked(authApi.resetPassword).mockRejectedValueOnce(undefined);

    const { useResetPassword } = await import('./useResetPassword');
    const { result } = renderHook(() => useResetPassword(), {
      wrapper: createWrapper(['/reset-password?token=abc']),
    });

    act(() => {
      result.current.onSubmit({ password: 'Sup3rSecret!' });
    });

    await waitFor(() => {
      expect(result.current.status).toBe('error');
    });
  });

  it('still reports the rate-limit error and stays on the form (not error) for a 429', async () => {
    const { authApi } = await import('@/features/auth/api/auth.api');
    const backendError = await makeConsumedHttpError(429, {});
    vi.mocked(authApi.resetPassword).mockRejectedValueOnce(backendError);

    const { useResetPassword } = await import('./useResetPassword');
    const { result } = renderHook(() => useResetPassword(), {
      wrapper: createWrapper(['/reset-password?token=abc']),
    });

    act(() => {
      result.current.onSubmit({ password: 'Sup3rSecret!' });
    });

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.rateLimitError);
    });
    expect(result.current.status).toBe('form');
  });

  it('disables submission while the reset request is pending', async () => {
    const { authApi } = await import('@/features/auth/api/auth.api');
    let resolveReset: (() => void) | undefined;
    vi.mocked(authApi.resetPassword).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveReset = () => {
          resolve({ message: 'ok' });
        };
      }),
    );

    const { useResetPassword } = await import('./useResetPassword');
    const { result } = renderHook(() => useResetPassword(), {
      wrapper: createWrapper(['/reset-password?token=abc']),
    });

    act(() => {
      result.current.onSubmit({ password: 'Sup3rSecret!' });
    });
    await waitFor(() => {
      expect(result.current.loading).toBe(true);
    });

    act(() => {
      resolveReset?.();
    });
    await waitFor(() => {
      expect(result.current.status).toBe('success');
    });
    expect(result.current.loading).toBe(false);
  });
});
