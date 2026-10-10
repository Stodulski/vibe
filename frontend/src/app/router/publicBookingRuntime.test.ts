// @vitest-environment happy-dom
// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import { TimeoutError } from 'ky';
import { server } from '@/test/msw/server';
import { makeConsumedHttpError } from '@/test/factories';
import { useStore } from '@/shared/stores';
import { captureException } from '@/shared/lib/observability';
import { createBookingClient } from '@vibe/booking/api-client';
import { getPublicBookingConfig, reportBookingError } from './publicBookingRuntime';

vi.mock('@/shared/lib/observability', () => ({ captureException: vi.fn() }));

const BASE = 'http://localhost/api/v1';

afterEach(() => {
  useStore.getState().setCsrfToken(null);
  vi.clearAllMocks();
});

describe('the booking client inside the app', () => {
  it('stays free of the session CSRF token even when the app holds one', async () => {
    useStore.getState().setCsrfToken('session-csrf');
    let csrfHeader: string | null | undefined;
    server.use(
      http.get(`${BASE}/ping`, ({ request }) => {
        csrfHeader = request.headers.get('X-CSRF-Token');
        return HttpResponse.json({});
      }),
    );

    await createBookingClient(BASE).get('ping').json();

    expect(csrfHeader).toBeNull();
  });
});

describe('reportBookingError', () => {
  it.each([401, 403, 404, 422])('does not report an expected %i', async (status) => {
    const error = await makeConsumedHttpError(status, {});

    reportBookingError(error);

    expect(captureException).not.toHaveBeenCalled();
  });

  it('reports a 5xx with its status and path as tags', async () => {
    const error = await makeConsumedHttpError(500, {});

    reportBookingError(error);

    expect(captureException).toHaveBeenCalledTimes(1);
    const [reported, context] = vi.mocked(captureException).mock.calls[0] ?? [];
    expect(reported).toBe(error);
    expect(context).toMatchObject({ tags: { status: 500, pathname: '/test' } });
  });

  it('reports a timeout, which never produces an HTTP response', () => {
    const error = new TimeoutError(new Request(`${BASE}/book`));

    reportBookingError(error);

    expect(captureException).toHaveBeenCalledWith(error);
  });
});

describe('getPublicBookingConfig', () => {
  it('resolves the API base to an absolute URL', () => {
    expect(getPublicBookingConfig().apiBaseUrl).toMatch(/^https?:\/\/.+\/api\/v1$/);
  });

  it('points the privacy link at /privacidad on the landing origin', () => {
    expect(getPublicBookingConfig().privacyUrl).toMatch(/\/privacidad$/);
  });

  it('wires its error reporter to reportBookingError', () => {
    expect(getPublicBookingConfig().reportError).toBe(reportBookingError);
  });
});
