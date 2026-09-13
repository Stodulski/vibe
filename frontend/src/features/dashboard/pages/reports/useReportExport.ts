import { useEffect, useRef, useState } from 'react';
import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { dashboardApi } from '@/features/dashboard';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getApiError, getHttpErrorMessage } from '@/shared/lib/utils';
import type { PaymentsExport } from '@/shared/types/api.types';

const t = ES_AR;

/** How often the status route is polled once a job is `pending`/`running`. */
const POLL_INTERVAL_MS = 1500;

/**
 * Upper bound on how long a job is followed before giving up client-side.
 * The backend's own worker budget is far shorter (10s, design doc §5); this
 * is just a backstop against a stuck job leaving the button spinning
 * forever.
 */
const POLL_TIMEOUT_MS = 120_000;

const ACTIVE_STATUSES = new Set<PaymentsExport['status']>(['pending', 'running']);

/**
 * Whether `POST …/reports/exports` failed the way an older or half-deployed
 * backend does, rather than the way a real refusal does: a bare 404 (the
 * route doesn't exist yet — JOB-06 not deployed here) or a 501 (the route
 * exists but no private object storage is configured, design doc §3). Both
 * mean "there is no async job to poll," not "the export failed" — the caller
 * falls back to the synchronous `exportPaymentsExcel` instead of showing an
 * error.
 */
function isLegacyExportRoute(error: unknown): boolean {
  return error instanceof HTTPError && (error.response.status === 404 || error.response.status === 501);
}

/** Whether `error` is the 410 the status route answers past the 24h retention window (design doc §5, kind `gone`). */
function isExportGone(error: unknown): boolean {
  return error instanceof HTTPError && error.response.status === 410;
}

/**
 * Triggers the browser's save-file dialog for an already-signed download URL.
 *
 * Unlike the synchronous export's blob, this URL is never fetched by the
 * client: it is a presigned R2 GET whose `ResponseContentDisposition` the
 * backend already set to `attachment` (design doc §3), so a plain anchor
 * navigation downloads it without ever needing R2 to answer a CORS
 * preflight the way reading its bytes into a blob would.
 */
function triggerDownload(url: string): void {
  const a = document.createElement('a');
  a.href = url;
  a.rel = 'noopener';
  a.click();
}

function downloadExcelBlob(blob: Blob, month: number, year: number): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `pagos_${String(month)}_${String(year)}.xlsx`;
  a.click();
  URL.revokeObjectURL(url);
}

/** The user-facing message for a job that settled as `failed`, or a poll request that itself failed. */
function jobErrorMessage(job: PaymentsExport | undefined, pollError: unknown): string | null {
  if (job?.status === 'failed') {
    return job.error ? getApiError(job.error, t.dashboard.exportGenericError) : t.dashboard.exportGenericError;
  }
  if (pollError) {
    return isExportGone(pollError)
      ? t.dashboard.exportExpired
      : getHttpErrorMessage(pollError, t.dashboard.exportGenericError);
  }
  return null;
}

/**
 * Polls one export job's status every {@link POLL_INTERVAL_MS} while it is
 * `pending`/`running`, stopping on a terminal status, a request failure, or
 * the {@link POLL_TIMEOUT_MS} backstop.
 *
 * Every derived value (`job`, `jobTerminal`, the error message) is computed
 * straight from render, not copied into its own `setState` — the query
 * itself is the state, so there is nothing to keep in sync and no
 * `react-hooks/set-state-in-effect` violation from an effect that would
 * otherwise exist only to mirror it. The one genuine side effect (opening
 * the signed download URL) stays in an effect that never calls `setState`.
 */
function useExportJobPoll(complexId: string, exportId: string | null) {
  const [timedOut, setTimedOut] = useState(false);
  const downloadedIdRef = useRef<string | null>(null);

  const pollQuery: UseQueryResult<PaymentsExport> = useQuery({
    queryKey: queryKeys.reportsExport.status(complexId, exportId ?? ''),
    queryFn: ({ signal }) => dashboardApi.getPaymentsExport(complexId, exportId ?? '', signal),
    select: (data) => data.export,
    enabled: exportId !== null,
    throwOnError: false,
    retry: false,
    refetchInterval: (query) => {
      if (timedOut || query.state.error) return false;
      // `query.state.data` is the raw, pre-`select` response shape
      // (`{ export: PaymentsExport }`), not the selected `PaymentsExport`
      // itself — `select` only runs for the hook's own subscribers.
      const status = query.state.data?.export.status;
      return status && !ACTIVE_STATUSES.has(status) ? false : POLL_INTERVAL_MS;
    },
  });

  const job = pollQuery.data;
  const terminal = timedOut || job?.status === 'done' || job?.status === 'failed' || pollQuery.error != null;

  useEffect(() => {
    if (exportId === null || terminal) return;
    const timer = setTimeout(() => {
      setTimedOut(true);
    }, POLL_TIMEOUT_MS);
    return () => {
      clearTimeout(timer);
    };
  }, [exportId, terminal]);

  useEffect(() => {
    if (job?.status === 'done' && job.download_url && downloadedIdRef.current !== exportId) {
      downloadedIdRef.current = exportId;
      triggerDownload(job.download_url);
    }
  }, [job, exportId]);

  return {
    following: exportId !== null && !terminal,
    error: timedOut ? t.dashboard.exportTimedOut : jobErrorMessage(job, pollQuery.error),
    reset: () => {
      setTimedOut(false);
      downloadedIdRef.current = null;
    },
  };
}

export function useReportExport(complexId: string, month: number, year: number) {
  const [exportId, setExportId] = useState<string | null>(null);
  const [postPending, setPostPending] = useState(false);
  const [syncPending, setSyncPending] = useState(false);
  const [manualError, setManualError] = useState<string | null>(null);

  const jobPoll = useExportJobPoll(complexId, exportId);

  const runSyncFallback = async () => {
    setSyncPending(true);
    try {
      const blob = await dashboardApi.exportPaymentsExcel(complexId, month, year);
      downloadExcelBlob(blob, month, year);
    } catch (error) {
      // `.blob()` still throws a `ky` HTTPError on a non-2xx response, and
      // its body must be read through `getHttpErrorMessage`, not discarded,
      // or a real server code (too large, timed out) reads as a generic
      // failure.
      setManualError(
        error instanceof HTTPError
          ? getHttpErrorMessage(error, t.dashboard.exportGenericError)
          : t.dashboard.exportGenericError,
      );
    } finally {
      setSyncPending(false);
    }
  };

  const handleExport = async () => {
    setManualError(null);
    jobPoll.reset();
    setExportId(null);
    setPostPending(true);
    try {
      const { export: created } = await dashboardApi.requestPaymentsExport(complexId, { month, year });
      setExportId(created.id);
    } catch (error) {
      if (isLegacyExportRoute(error)) {
        await runSyncFallback();
      } else {
        setManualError(getHttpErrorMessage(error, t.dashboard.exportGenericError));
      }
    } finally {
      setPostPending(false);
    }
  };

  return {
    exporting: postPending || syncPending || jobPoll.following,
    exportError: manualError ?? jobPoll.error,
    // The async job's own copy while a job id is being followed, the sync
    // fallback's while that flow (the initial POST, or the legacy blob
    // download) is in flight instead.
    statusLabel: jobPoll.following ? t.dashboard.exportGenerating : t.reports.downloading,
    handleExport,
  };
}
