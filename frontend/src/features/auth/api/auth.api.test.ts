import { describe, it, expect, vi } from 'vitest';
// @vitest-environment node
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { authApi } from './auth.api';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { makeUser } from '@/test/factories';

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

describe('authApi', () => {
  it('login parses a complete AuthResponse', async () => {
    let receivedBody: unknown;
    server.use(
      http.post('*/auth/login', async ({ request }) => {
        receivedBody = await request.json();
        return HttpResponse.json({ user: makeUser(), csrf_token: 'tok' });
      }),
    );

    const data = { email: 'juan@test.com', password: 'secret' };
    const result = await authApi.login(data);

    expect(receivedBody).toEqual(data);
    expect(result).toEqual({ user: makeUser(), csrf_token: 'tok' });
  });

  it('login rejects with ApiResponseError when the response body does not match the schema', async () => {
    server.use(http.post('*/auth/login', () => HttpResponse.json({ user: { id: 'u1' } })));

    await expect(authApi.login({ email: 'juan@test.com', password: 'secret' })).rejects.toThrow(ApiResponseError);
  });

  it('register parses a message response', async () => {
    server.use(http.post('*/auth/register', () => HttpResponse.json({ message: 'ok' })));

    const data = {
      first_name: 'Juan',
      last_name: 'Garcia',
      email: 'juan@test.com',
      password: 'secret',
      phone: '1155550000',
    };
    await expect(authApi.register(data)).resolves.toEqual({ message: 'ok' });
  });

  it('verifyEmail parses a message response', async () => {
    await expect(authApi.verifyEmail('tok')).resolves.toEqual({ message: 'ok' });
  });

  it('resendVerification parses a message response', async () => {
    await expect(authApi.resendVerification('juan@test.com')).resolves.toEqual({ message: 'ok' });
  });

  it('forgotPassword parses a message response', async () => {
    await expect(authApi.forgotPassword('juan@test.com')).resolves.toEqual({ message: 'ok' });
  });

  it('resetPassword parses a message response', async () => {
    await expect(authApi.resetPassword('tok', 'newpass')).resolves.toEqual({ message: 'ok' });
  });

  it('deleteAccount parses a message response', async () => {
    await expect(authApi.deleteAccount()).resolves.toEqual({ message: 'ok' });
  });
});

// Sibling describes, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body.
describe('authApi.updateMe', () => {
  it('parses the account envelope, including a null pending_email and email_change "none"', async () => {
    server.use(
      http.put('*/auth/me', () => HttpResponse.json({ user: makeUser(), pending_email: null, email_change: 'none' })),
    );
    await expect(authApi.updateMe({ first_name: 'Juan' })).resolves.toEqual({
      user: makeUser(),
      pending_email: null,
      email_change: 'none',
    });
  });

  it('parses a non-null pending_email and email_change "requested", when the change hits a still-unconfirmed request', async () => {
    server.use(
      http.put('*/auth/me', () =>
        HttpResponse.json({ user: makeUser(), pending_email: 'new@test.com', email_change: 'requested' }),
      ),
    );
    await expect(authApi.updateMe({ email: 'new@test.com' })).resolves.toEqual({
      user: makeUser(),
      pending_email: 'new@test.com',
      email_change: 'requested',
    });
  });

  it('parses email_change "failed" alongside the untouched older pending_email', async () => {
    server.use(
      http.put('*/auth/me', () =>
        HttpResponse.json({ user: makeUser(), pending_email: 'old@test.com', email_change: 'failed' }),
      ),
    );
    await expect(authApi.updateMe({ email: 'new@test.com' })).resolves.toEqual({
      user: makeUser(),
      pending_email: 'old@test.com',
      email_change: 'failed',
    });
  });

  it('rejects an answer with no email_change field', async () => {
    server.use(http.put('*/auth/me', () => HttpResponse.json({ user: makeUser(), pending_email: null })));
    await expect(authApi.updateMe({ first_name: 'Juan' })).rejects.toThrow(ApiResponseError);
  });
});

describe('authApi.getMe', () => {
  it('parses the user, the CSRF token the session boots from, and pending_email', async () => {
    server.use(
      http.get('*/auth/me', () => HttpResponse.json({ user: makeUser(), csrf_token: 'tok', pending_email: null })),
    );
    await expect(authApi.getMe()).resolves.toEqual({ user: makeUser(), csrf_token: 'tok', pending_email: null });
  });

  it('rejects an answer with no csrf_token, since the boot path cannot run without one', async () => {
    server.use(http.get('*/auth/me', () => HttpResponse.json({ user: makeUser(), pending_email: null })));
    await expect(authApi.getMe()).rejects.toThrow(ApiResponseError);
  });

  it('rejects an answer with no pending_email field', async () => {
    server.use(http.get('*/auth/me', () => HttpResponse.json({ user: makeUser(), csrf_token: 'tok' })));
    await expect(authApi.getMe()).rejects.toThrow(ApiResponseError);
  });
});

describe('authApi.confirmEmailChange', () => {
  it('posts the token and parses a message response', async () => {
    let receivedBody: unknown;
    server.use(
      http.post('*/auth/confirm-email-change', async ({ request }) => {
        receivedBody = await request.json();
        return HttpResponse.json({ message: 'email address updated' });
      }),
    );

    await expect(authApi.confirmEmailChange('tok')).resolves.toEqual({ message: 'email address updated' });
    expect(receivedBody).toEqual({ token: 'tok' });
  });
});
