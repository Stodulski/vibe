// @vitest-environment node
const { mockGet, mockPost, mockPut, mockDelete } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn(),
}));

vi.mock('@/shared/lib/ky', () => ({
  default: { get: mockGet, post: mockPost, put: mockPut, delete: mockDelete },
  withSignal: (signal?: AbortSignal) => (signal ? { signal } : {}),
}));

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

import { authApi } from './auth.api';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { makeUser } from '@/test/factories';
import type { LoginRequest, RegisterRequest, UpdateMeRequest } from '@/shared/types/api.types';

function mockJsonOnce(mock: typeof mockGet, data: unknown) {
  mock.mockReturnValueOnce({ json: vi.fn().mockResolvedValue(data) });
}

describe('authApi', () => {
  beforeEach(() => {
    mockGet.mockClear();
    mockPost.mockClear();
    mockPut.mockClear();
    mockDelete.mockClear();
  });

  it('login parses a complete AuthResponse', async () => {
    const data: LoginRequest = { email: 'juan@test.com', password: 'secret' };
    mockJsonOnce(mockPost, { user: makeUser(), csrf_token: 'tok' });

    const result = await authApi.login(data);

    expect(mockPost).toHaveBeenCalledWith('auth/login', { json: data });
    expect(result).toEqual({ user: makeUser(), csrf_token: 'tok' });
  });

  it('login rejects with ApiResponseError when the response body does not match the schema', async () => {
    mockJsonOnce(mockPost, { user: { id: 'u1' } });

    await expect(authApi.login({ email: 'juan@test.com', password: 'secret' })).rejects.toThrow(ApiResponseError);
  });

  it('register parses a message response', async () => {
    const data: RegisterRequest = {
      first_name: 'Juan',
      last_name: 'Garcia',
      email: 'juan@test.com',
      password: 'secret',
      phone: '1155550000',
    };
    mockJsonOnce(mockPost, { message: 'ok' });

    await expect(authApi.register(data)).resolves.toEqual({ message: 'ok' });
  });

  it('verifyEmail parses a message response', async () => {
    mockJsonOnce(mockPost, { message: 'ok' });
    await expect(authApi.verifyEmail('tok')).resolves.toEqual({ message: 'ok' });
  });

  it('resendVerification parses a message response', async () => {
    mockJsonOnce(mockPost, { message: 'ok' });
    await expect(authApi.resendVerification('juan@test.com')).resolves.toEqual({ message: 'ok' });
  });

  it('forgotPassword parses a message response', async () => {
    mockJsonOnce(mockPost, { message: 'ok' });
    await expect(authApi.forgotPassword('juan@test.com')).resolves.toEqual({ message: 'ok' });
  });

  it('resetPassword parses a message response', async () => {
    mockJsonOnce(mockPost, { message: 'ok' });
    await expect(authApi.resetPassword('tok', 'newpass')).resolves.toEqual({ message: 'ok' });
  });

  it('getMe parses a user envelope', async () => {
    mockJsonOnce(mockGet, { user: makeUser() });
    await expect(authApi.getMe()).resolves.toEqual({ user: makeUser() });
  });

  it('updateMe parses a user envelope', async () => {
    const data: UpdateMeRequest = { first_name: 'Juan' };
    mockPut.mockReturnValueOnce({ json: vi.fn().mockResolvedValue({ user: makeUser() }) });
    await expect(authApi.updateMe(data)).resolves.toEqual({ user: makeUser() });
  });

  it('deleteAccount parses a message response', async () => {
    mockDelete.mockReturnValueOnce({ json: vi.fn().mockResolvedValue({ message: 'ok' }) });
    await expect(authApi.deleteAccount()).resolves.toEqual({ message: 'ok' });
  });
});
