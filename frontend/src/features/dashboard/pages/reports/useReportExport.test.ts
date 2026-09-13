import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { MockInstance } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { createQueryWrapper } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * Every scenario below drives the real `ky` client through MSW (see
 * `src/test/msw/handlers.ts`'s JOB-06 defaults) rather than mocking
 * `dashboardApi`, so the POST → poll → download chain — and the fallback's
 * own error mapping — run through the actual `getHttpErrorMessage`/`getApiError`
 * paths, not a stand-in.
 *
 * `HTMLAnchorElement.prototype.click` is mocked so the download step never
 * touches real navigation: `useReportExport` builds a plain `<a>` and clicks
 * it for both the signed `download_url` (async job) and the `blob:` object
 * URL (sync fallback), and the mock captures whichever `href` it saw.
 */
let downloadedHref: string | undefined;
let clickSpy: MockInstance<(this: HTMLAnchorElement) => void>;

beforeEach(() => {
  downloadedHref = undefined;
  clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
    downloadedHref = this.href;
  });
});

afterEach(() => {
  clickSpy.mockRestore();
});

describe('useReportExport — async export job (JOB-06) happy path', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('polls pending → running → done and downloads the signed URL', async () => {
    let call = 0;
    server.use(
      http.post('*/complexes/:complexId/reports/exports', () =>
        HttpResponse.json({ export: { id: 'exp-1', status: 'pending', status_url: '/x' } }, { status: 202 }),
      ),
      http.get('*/complexes/:complexId/reports/exports/:exportId', () => {
        call += 1;
        if (call === 1) return HttpResponse.json({ export: { id: 'exp-1', status: 'pending' } });
        if (call === 2) return HttpResponse.json({ export: { id: 'exp-1', status: 'running' } });
        return HttpResponse.json({
          export: { id: 'exp-1', status: 'done', download_url: 'https://r2.test/exp-1.xlsx' },
        });
      }),
    );

    const { useReportExport } = await import('./useReportExport');
    const { result } = renderHook(() => useReportExport('c1', 3, 2026), { wrapper: createQueryWrapper() });

    await act(async () => {
      await result.current.handleExport();
    });

    await waitFor(() => {
      expect(call).toBeGreaterThanOrEqual(1);
    });
    expect(result.current.exporting).toBe(true);

    // Each poll is 1.5s apart (`POLL_INTERVAL_MS`); two ticks carry
    // pending → running → done.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    await waitFor(() => {
      expect(result.current.exporting).toBe(false);
    });
    expect(call).toBe(3);
    expect(downloadedHref).toBe('https://r2.test/exp-1.xlsx');
    expect(result.current.exportError).toBeNull();
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call (see useBookingStatus.test.ts's own comment).
describe('useReportExport — cancels polling on unmount', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('cancels polling once the component unmounts', async () => {
    let call = 0;
    server.use(
      http.post('*/complexes/:complexId/reports/exports', () =>
        HttpResponse.json({ export: { id: 'exp-4', status: 'pending', status_url: '/x' } }, { status: 202 }),
      ),
      http.get('*/complexes/:complexId/reports/exports/:exportId', () => {
        call += 1;
        return HttpResponse.json({ export: { id: 'exp-4', status: 'pending' } });
      }),
    );

    const { useReportExport } = await import('./useReportExport');
    const { result, unmount } = renderHook(() => useReportExport('c1', 3, 2026), { wrapper: createQueryWrapper() });

    await act(async () => {
      await result.current.handleExport();
    });

    await waitFor(() => {
      expect(call).toBe(1);
    });

    unmount();
    const callsAtUnmount = call;

    // Past two poll intervals: if the query kept following the job after
    // unmount, `call` would grow past this — it doesn't, because
    // unsubscribing the last observer of a `gcTime: 0` query (see
    // `createQueryWrapper`) drops its `refetchInterval` scheduling.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(4000);
    });

    expect(call).toBe(callsAtUnmount);
  });
});

describe('useReportExport — job settles as failed or gone', () => {
  it('reads the embedded Problem when the job settles as failed', async () => {
    server.use(
      http.post('*/complexes/:complexId/reports/exports', () =>
        HttpResponse.json({ export: { id: 'exp-2', status: 'pending', status_url: '/x' } }, { status: 202 }),
      ),
      http.get('*/complexes/:complexId/reports/exports/:exportId', () =>
        HttpResponse.json({
          export: {
            id: 'exp-2',
            status: 'failed',
            error: {
              type: 'https://vibe.com.ar/problems/unavailable',
              title: 'Service Unavailable',
              status: 500,
              detail: 'report_export_too_large',
            },
          },
        }),
      ),
    );

    const { useReportExport } = await import('./useReportExport');
    const { result } = renderHook(() => useReportExport('c1', 3, 2026), { wrapper: createQueryWrapper() });

    await act(async () => {
      await result.current.handleExport();
    });

    await waitFor(() => {
      expect(result.current.exportError).toBe(t.dashboard.exportTooLarge);
    });
    expect(result.current.exporting).toBe(false);
  });

  it('shows the expired message on a 410 once the file has passed its retention', async () => {
    server.use(
      http.post('*/complexes/:complexId/reports/exports', () =>
        HttpResponse.json({ export: { id: 'exp-3', status: 'pending', status_url: '/x' } }, { status: 202 }),
      ),
      http.get('*/complexes/:complexId/reports/exports/:exportId', () =>
        HttpResponse.json({ type: 'https://vibe.com.ar/problems/gone', title: 'Gone', status: 410 }, { status: 410 }),
      ),
    );

    const { useReportExport } = await import('./useReportExport');
    const { result } = renderHook(() => useReportExport('c1', 3, 2026), { wrapper: createQueryWrapper() });

    await act(async () => {
      await result.current.handleExport();
    });

    await waitFor(() => {
      expect(result.current.exportError).toBe(t.dashboard.exportExpired);
    });
    expect(result.current.exporting).toBe(false);
  });
});

describe('useReportExport — falls back to the synchronous export', () => {
  it('falls back on a 501 (no object storage backend configured)', async () => {
    server.use(
      http.post('*/complexes/:complexId/reports/exports', () =>
        HttpResponse.json(
          { type: 'https://vibe.com.ar/problems/unavailable', title: 'Not Implemented', status: 501 },
          { status: 501 },
        ),
      ),
    );

    const { useReportExport } = await import('./useReportExport');
    const { result } = renderHook(() => useReportExport('c1', 3, 2026), { wrapper: createQueryWrapper() });

    await act(async () => {
      await result.current.handleExport();
    });

    await waitFor(() => {
      expect(result.current.exporting).toBe(false);
    });
    expect(downloadedHref).toMatch(/^blob:/);
    expect(result.current.exportError).toBeNull();
  });

  it('falls back on a bare 404 (the async routes do not exist yet)', async () => {
    server.use(http.post('*/complexes/:complexId/reports/exports', () => HttpResponse.json(null, { status: 404 })));

    const { useReportExport } = await import('./useReportExport');
    const { result } = renderHook(() => useReportExport('c1', 3, 2026), { wrapper: createQueryWrapper() });

    await act(async () => {
      await result.current.handleExport();
    });

    await waitFor(() => {
      expect(result.current.exporting).toBe(false);
    });
    expect(downloadedHref).toMatch(/^blob:/);
    expect(result.current.exportError).toBeNull();
  });
});

// The pre-JOB-06 test suite: adapted to trigger through the fallback (a 501
// from the job route) instead of mocking `exportPaymentsExcel` directly, so
// the sync path's own `report_export_*` code mapping stays covered end to
// end via MSW rather than a stand-in for `dashboardApi`.
describe('useReportExport — sync fallback maps report_export_* codes instead of a generic fallback', () => {
  beforeEach(() => {
    server.use(
      http.post('*/complexes/:complexId/reports/exports', () =>
        HttpResponse.json(
          { type: 'https://vibe.com.ar/problems/unavailable', title: 'Not Implemented', status: 501 },
          { status: 501 },
        ),
      ),
    );
  });

  it('shows the "too large" message when the server refuses report_export_too_large', async () => {
    server.use(
      http.get('*/complexes/:complexId/reports/export', () =>
        HttpResponse.json({ title: 'Unprocessable Entity', detail: 'report_export_too_large' }, { status: 422 }),
      ),
    );

    const { useReportExport } = await import('./useReportExport');
    const { result } = renderHook(() => useReportExport('c1', 3, 2026), { wrapper: createQueryWrapper() });

    await act(async () => {
      await result.current.handleExport();
    });

    await waitFor(() => {
      expect(result.current.exportError).not.toBeNull();
    });
    expect(result.current.exportError).toBe(t.dashboard.exportTooLarge);
  });

  it('shows the "timed out" message when the server refuses report_export_timed_out', async () => {
    server.use(
      http.get('*/complexes/:complexId/reports/export', () =>
        HttpResponse.json({ title: 'Service Unavailable', detail: 'report_export_timed_out' }, { status: 503 }),
      ),
    );

    const { useReportExport } = await import('./useReportExport');
    const { result } = renderHook(() => useReportExport('c1', 3, 2026), { wrapper: createQueryWrapper() });

    await act(async () => {
      await result.current.handleExport();
    });

    await waitFor(() => {
      expect(result.current.exportError).not.toBeNull();
    });
    expect(result.current.exportError).toBe(t.dashboard.exportTimedOut);
  });

  it('falls back to the generic message for an unrecognized code', async () => {
    server.use(http.get('*/complexes/:complexId/reports/export', () => HttpResponse.json({}, { status: 500 })));

    const { useReportExport } = await import('./useReportExport');
    const { result } = renderHook(() => useReportExport('c1', 3, 2026), { wrapper: createQueryWrapper() });

    await act(async () => {
      await result.current.handleExport();
    });

    await waitFor(() => {
      expect(result.current.exportError).not.toBeNull();
    });
    expect(result.current.exportError).toBe(t.dashboard.exportGenericError);
  });
});
