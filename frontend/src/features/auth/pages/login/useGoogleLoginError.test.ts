import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { createWrapper } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useGoogleLoginError } from './useGoogleLoginError';

const t = ES_AR;

function renderAt(path: string) {
  return renderHook(() => useGoogleLoginError(), { wrapper: createWrapper([path]) });
}

describe('useGoogleLoginError', () => {
  it('maps google_rejected to its own copy', () => {
    const { result } = renderAt('/login?error=google_rejected');
    expect(result.current).toBe(t.auth.googleErrorRejected);
  });

  it('maps google_unavailable to its own copy', () => {
    const { result } = renderAt('/login?error=google_unavailable');
    expect(result.current).toBe(t.auth.googleErrorUnavailable);
  });

  it('maps google_expired to its own copy', () => {
    const { result } = renderAt('/login?error=google_expired');
    expect(result.current).toBe(t.auth.googleErrorExpired);
  });

  // The rate-limit copy is shared by both the redirect handler's own 429 and
  // the exchange's — both land here as the same query value.
  it('maps google_rate_limited to its own copy', () => {
    const { result } = renderAt('/login?error=google_rate_limited');
    expect(result.current).toBe(t.auth.googleErrorRateLimited);
  });

  it('shows nothing for an error value that is not one of ours', () => {
    const { result } = renderAt('/login?error=something_else');
    expect(result.current).toBeUndefined();
  });

  it('shows nothing when there is no error at all', () => {
    const { result } = renderAt('/login');
    expect(result.current).toBeUndefined();
  });
});
