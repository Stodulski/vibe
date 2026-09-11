import { renderHook, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import { createWrapper } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('../api/auth.api', () => ({
  authApi: {
    register: vi.fn(),
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

/** Mutates once with a rejected `authApi.register` and waits for the error state. */
async function triggerRegisterError(backendError: unknown, options?: { resetTurnstile?: () => void }) {
  const { authApi } = await import('../api/auth.api');
  vi.mocked(authApi.register).mockRejectedValueOnce(backendError);

  const { useRegister } = await import('./useRegister');
  const { result } = renderHook(() => useRegister(options), { wrapper: createWrapper(['/register']) });

  result.current.mutate({
    first_name: 'Juan',
    last_name: 'Perez',
    email: 'juan@test.com',
    password: 'password123',
    phone: '+541123456789',
  });

  await waitFor(() => {
    expect(result.current.isError).toBe(true);
  });
}

describe('useRegister — Turnstile 422 handling', () => {
  it('shows the turnstile-specific message instead of the generic register-error one', async () => {
    await triggerRegisterError(await makeConsumedHttpError(422, { error: { turnstile_token: 'required' } }));

    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.turnstileRequired);
    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.auth.registerError);
  });

  it('translates every documented turnstile_token code', async () => {
    await triggerRegisterError(await makeConsumedHttpError(422, { error: { turnstile_token: 'invalid' } }));
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.turnstileInvalid);

    await triggerRegisterError(await makeConsumedHttpError(422, { error: { turnstile_token: 'unavailable' } }));
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.turnstileUnavailable);
  });

  it('resets the widget on a turnstile failure — tokens are single-use', async () => {
    const resetTurnstile = vi.fn();

    await triggerRegisterError(await makeConsumedHttpError(422, { error: { turnstile_token: 'invalid' } }), {
      resetTurnstile,
    });

    expect(resetTurnstile).toHaveBeenCalledTimes(1);
  });

  it('also resets the widget on an unrelated failure — a solved token is burned by any failed submit', async () => {
    const resetTurnstile = vi.fn();

    await triggerRegisterError(await makeConsumedHttpError(500, {}), { resetTurnstile });

    expect(resetTurnstile).toHaveBeenCalledTimes(1);
  });

  it('still shows the rate-limit message on a 429, unaffected by the turnstile check', async () => {
    await triggerRegisterError(await makeConsumedHttpError(429, {}));

    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.rateLimitError);
  });

  afterEach(async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.register).mockReset();
  });
});
