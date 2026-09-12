// @vitest-environment node
import { HTTPError } from 'ky';
import { bootstrapSession, loginUrlPreserving, refreshAccessToken, REFRESH_RETRY_DELAY_MS } from './ky';
import { makeUser } from '@/test/factories';

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

vi.mock('ky', () => {
  const mockPost = vi.fn().mockReturnValue({
    json: vi.fn().mockResolvedValue({ csrf_token: 'new-csrf-token' }),
  });
  // The real class carries a Response; `bootstrapSession` only reads
  // `response.status`, which is all this stand-in needs to offer.
  class MockHTTPError extends Error {
    response: { status: number };
    constructor(status: number) {
      super(`HTTP ${String(status)}`);
      this.response = { status };
    }
  }
  return {
    default: {
      create: vi.fn().mockReturnValue({}),
      post: mockPost,
      get: vi.fn(),
    },
    HTTPError: MockHTTPError,
  };
});

const mockSetCsrfToken = vi.fn();
const mockLogout = vi.fn();
vi.mock('@/shared/stores', () => ({
  useStore: {
    getState: () => ({
      csrfToken: 'test-csrf',
      setCsrfToken: mockSetCsrfToken,
      logout: mockLogout,
    }),
  },
}));

describe('refreshAccessToken', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('calls setCsrfToken with the new token', async () => {
    await refreshAccessToken();
    expect(mockSetCsrfToken).toHaveBeenCalledWith('new-csrf-token');
  });

  it('deduplicates concurrent calls', async () => {
    const p1 = refreshAccessToken();
    const p2 = refreshAccessToken();
    await Promise.all([p1, p2]);
    // ky.post should have been called only once despite two calls
    const ky = await import('ky');
    expect(ky.default.post).toHaveBeenCalledTimes(1);
  });
});

describe('refreshAccessToken — a sibling tab refreshed first', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('retries once after a pause, and takes the token from the retry', async () => {
    const ky = await import('ky');
    vi.mocked(ky.default.post)
      .mockReturnValueOnce({ json: vi.fn().mockRejectedValue(new Error('401')) } as never)
      .mockReturnValueOnce({
        json: vi.fn().mockResolvedValue({ csrf_token: 'from-retry' }),
      } as never);

    const pending = refreshAccessToken();
    await vi.advanceTimersByTimeAsync(REFRESH_RETRY_DELAY_MS);
    await pending;

    expect(ky.default.post).toHaveBeenCalledTimes(2);
    expect(mockSetCsrfToken).toHaveBeenCalledWith('from-retry');
  });

  it('gives up after the retry also fails', async () => {
    const ky = await import('ky');
    vi.mocked(ky.default.post)
      .mockReturnValueOnce({ json: vi.fn().mockRejectedValue(new Error('401')) } as never)
      .mockReturnValueOnce({ json: vi.fn().mockRejectedValue(new Error('401')) } as never);

    const pending = refreshAccessToken();
    // Attach the rejection handler before the timers run, or the rejection is
    // reported as unhandled while the clock advances.
    const outcome = pending.then(
      () => 'resolved',
      () => 'rejected',
    );
    await vi.advanceTimersByTimeAsync(REFRESH_RETRY_DELAY_MS);

    await expect(outcome).resolves.toBe('rejected');
    expect(ky.default.post).toHaveBeenCalledTimes(2);
    expect(mockSetCsrfToken).not.toHaveBeenCalled();
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call. Its own `beforeEach`/`afterEach` duplicate the
// fake-timer setup above rather than sharing it, for the same reason.
describe('refreshAccessToken — malformed refresh body', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // A malformed refresh body (e.g. no `csrf_token`) must not silently flow
  // into `setCsrfToken(undefined)` — `postRefresh` validates it against
  // `refreshResponseSchema` and throws `ApiResponseError`, which needs to
  // take the exact same path as a rejected (401) refresh: one retry, and if
  // the retry is malformed too, the same "give up" outcome as two 401s.
  it('treats a malformed refresh body (missing csrf_token) exactly like a failed refresh: retries once, then gives up', async () => {
    const ky = await import('ky');
    vi.mocked(ky.default.post)
      .mockReturnValueOnce({ json: vi.fn().mockResolvedValue({}) } as never)
      .mockReturnValueOnce({ json: vi.fn().mockResolvedValue({}) } as never);

    const pending = refreshAccessToken();
    const outcome = pending.then(
      () => 'resolved',
      () => 'rejected',
    );
    await vi.advanceTimersByTimeAsync(REFRESH_RETRY_DELAY_MS);

    await expect(outcome).resolves.toBe('rejected');
    expect(ky.default.post).toHaveBeenCalledTimes(2);
    expect(mockSetCsrfToken).not.toHaveBeenCalled();
  });

  it('recovers from one malformed refresh body if the retry comes back valid', async () => {
    const ky = await import('ky');
    vi.mocked(ky.default.post)
      .mockReturnValueOnce({ json: vi.fn().mockResolvedValue({}) } as never)
      .mockReturnValueOnce({
        json: vi.fn().mockResolvedValue({ csrf_token: 'from-retry' }),
      } as never);

    const pending = refreshAccessToken();
    await vi.advanceTimersByTimeAsync(REFRESH_RETRY_DELAY_MS);
    await pending;

    expect(ky.default.post).toHaveBeenCalledTimes(2);
    expect(mockSetCsrfToken).toHaveBeenCalledWith('from-retry');
  });
});

// `bootstrapSession` is the boot path: GET /auth/me first, and the refresh
// token is only spent once that answered 401. It bypasses the `api` instance,
// so the 401 hook's logout-and-redirect never runs for an anonymous visitor.
describe('bootstrapSession', () => {
  const user = makeUser();
  const session = { user, csrf_token: 'from-me' };
  // The mocked constructor above takes a status, unlike the real one.
  const MockedHTTPError = HTTPError as unknown as new (status: number) => HTTPError;
  const status401 = () => new MockedHTTPError(401);

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    vi.stubGlobal('window', { location: { pathname: '/', href: '/' } });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('reads the session from /auth/me and never calls /auth/refresh while the access token is alive', async () => {
    const ky = await import('ky');
    vi.mocked(ky.default.get).mockReturnValueOnce({ json: vi.fn().mockResolvedValue(session) } as never);

    await expect(bootstrapSession()).resolves.toEqual(session);

    expect(ky.default.get).toHaveBeenCalledTimes(1);
    expect(ky.default.post).not.toHaveBeenCalled();
    expect(mockSetCsrfToken).not.toHaveBeenCalled();
  });

  it('refreshes once /auth/me answers 401, then reads the session again', async () => {
    const ky = await import('ky');
    vi.mocked(ky.default.get)
      .mockReturnValueOnce({ json: vi.fn().mockRejectedValue(status401()) } as never)
      .mockReturnValueOnce({ json: vi.fn().mockResolvedValue(session) } as never);
    vi.mocked(ky.default.post).mockReturnValueOnce({
      json: vi.fn().mockResolvedValue({ csrf_token: 'from-refresh' }),
    } as never);

    await expect(bootstrapSession()).resolves.toEqual(session);

    expect(ky.default.get).toHaveBeenCalledTimes(2);
    expect(ky.default.post).toHaveBeenCalledTimes(1);
    expect(mockSetCsrfToken).toHaveBeenCalledWith('from-refresh');
  });

  it('answers null, without logging out or redirecting, when no refresh can recover the 401', async () => {
    const ky = await import('ky');
    vi.mocked(ky.default.get).mockReturnValueOnce({ json: vi.fn().mockRejectedValue(status401()) } as never);
    vi.mocked(ky.default.post)
      .mockReturnValueOnce({ json: vi.fn().mockRejectedValue(status401()) } as never)
      .mockReturnValueOnce({ json: vi.fn().mockRejectedValue(status401()) } as never);

    const pending = bootstrapSession();
    await vi.advanceTimersByTimeAsync(REFRESH_RETRY_DELAY_MS);

    await expect(pending).resolves.toBeNull();
    expect(ky.default.get).toHaveBeenCalledTimes(1);
    expect(ky.default.post).toHaveBeenCalledTimes(2);
    expect(mockLogout).not.toHaveBeenCalled();
    expect(window.location.href).toBe('/');
  });

  it('propagates any failure that is not a 401', async () => {
    const ky = await import('ky');
    vi.mocked(ky.default.get).mockReturnValueOnce({
      json: vi.fn().mockRejectedValue(new MockedHTTPError(503)),
    } as never);

    await expect(bootstrapSession()).rejects.toThrow('HTTP 503');
    expect(ky.default.post).not.toHaveBeenCalled();
  });
});

/**
 * A 401 that no refresh can recover ends the session with a hard navigation —
 * the store is gone, so React never renders a `<Navigate>` and router state
 * cannot carry the destination. `ProtectedRoute` has always sent `from` along
 * for the redirects it owns; this path used to drop it, so an access token
 * expiring mid-session cost the person the page they were reading on top of
 * making them log in again.
 */
describe('loginUrlPreserving', () => {
  it('keeps the path the person was on', () => {
    expect(loginUrlPreserving({ pathname: '/bookings', search: '' })).toBe('/login?from=%2Fbookings');
  });

  it('keeps the query string too — a booking list is a date, not just a route', () => {
    expect(loginUrlPreserving({ pathname: '/bookings', search: '?date=2026-03-18' })).toBe(
      '/login?from=%2Fbookings%3Fdate%3D2026-03-18',
    );
  });

  it('encodes the destination so it cannot break out of the query parameter', () => {
    const url = loginUrlPreserving({ pathname: '/admin/users', search: '?q=a&b=c' });
    expect(url).toBe('/login?from=%2Fadmin%2Fusers%3Fq%3Da%26b%3Dc');
    expect(new URLSearchParams(url?.split('?')[1]).get('from')).toBe('/admin/users?q=a&b=c');
  });

  it('does nothing on /login or /register, where a redirect would only wipe a half-typed form', () => {
    expect(loginUrlPreserving({ pathname: '/login', search: '' })).toBeNull();
    expect(loginUrlPreserving({ pathname: '/register', search: '?ref=x' })).toBeNull();
  });
});
