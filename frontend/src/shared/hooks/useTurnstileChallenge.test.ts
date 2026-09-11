import { act, renderHook } from '@testing-library/react';
import { useTurnstileChallenge } from './useTurnstileChallenge';

const mockEnv = vi.hoisted((): { VITE_TURNSTILE_SITE_KEY: string | undefined } => ({
  VITE_TURNSTILE_SITE_KEY: undefined,
}));
vi.mock('@/shared/lib/env', () => ({ env: mockEnv }));

describe('useTurnstileChallenge', () => {
  afterEach(() => {
    mockEnv.VITE_TURNSTILE_SITE_KEY = undefined;
  });

  it('is not required and never blocks submit when no site key is configured', () => {
    const { result } = renderHook(() => useTurnstileChallenge());

    expect(result.current.required).toBe(false);
    expect(result.current.isBlocked(false)).toBe(false);
    expect(result.current.payload()).toEqual({});
  });

  it('blocks submit until a token is set when a site key is configured', () => {
    mockEnv.VITE_TURNSTILE_SITE_KEY = 'test-site-key';
    const { result } = renderHook(() => useTurnstileChallenge());

    expect(result.current.required).toBe(true);
    expect(result.current.isBlocked(false)).toBe(true);

    act(() => {
      result.current.onTokenChange('solved-token');
    });

    expect(result.current.isBlocked(false)).toBe(false);
    expect(result.current.payload()).toEqual({ turnstile_token: 'solved-token' });
  });

  it('still blocks submit while the mutation itself is pending, token or not', () => {
    mockEnv.VITE_TURNSTILE_SITE_KEY = 'test-site-key';
    const { result } = renderHook(() => useTurnstileChallenge());

    act(() => {
      result.current.onTokenChange('solved-token');
    });

    expect(result.current.isBlocked(true)).toBe(true);
  });

  it('omits turnstile_token from the payload entirely (not present-as-undefined) with no token', () => {
    const { result } = renderHook(() => useTurnstileChallenge());

    expect('turnstile_token' in result.current.payload()).toBe(false);
  });
});
