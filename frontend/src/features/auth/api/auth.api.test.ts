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

  it('updateMe parses a user envelope', async () => {
    server.use(http.put('*/auth/me', () => HttpResponse.json({ user: makeUser() })));
    await expect(authApi.updateMe({ first_name: 'Juan' })).resolves.toEqual({ user: makeUser() });
  });

  it('deleteAccount parses a message response', async () => {
    await expect(authApi.deleteAccount()).resolves.toEqual({ message: 'ok' });
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body.
describe('authApi.getMe', () => {
  it('parses the user and the CSRF token the session boots from', async () => {
    server.use(http.get('*/auth/me', () => HttpResponse.json({ user: makeUser(), csrf_token: 'tok' })));
    await expect(authApi.getMe()).resolves.toEqual({ user: makeUser(), csrf_token: 'tok' });
  });

  it('rejects an answer with no csrf_token, since the boot path cannot run without one', async () => {
    server.use(http.get('*/auth/me', () => HttpResponse.json({ user: makeUser() })));
    await expect(authApi.getMe()).rejects.toThrow(ApiResponseError);
  });
});
