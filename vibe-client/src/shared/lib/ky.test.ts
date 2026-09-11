// @vitest-environment node
import { refreshAccessToken, REFRESH_RETRY_DELAY_MS } from './ky';

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

vi.mock('ky', () => {
  const mockPost = vi.fn().mockReturnValue({
    json: vi.fn().mockResolvedValue({ csrf_token: 'new-csrf-token' }),
  });
  return {
    default: {
      create: vi.fn().mockReturnValue({}),
      post: mockPost,
    },
  };
});

const mockSetCsrfToken = vi.fn();
vi.mock('@/shared/stores', () => ({
  useStore: {
    getState: () => ({
      csrfToken: 'test-csrf',
      setCsrfToken: mockSetCsrfToken,
      logout: vi.fn(),
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
