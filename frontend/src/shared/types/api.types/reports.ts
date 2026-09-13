import type { Ok, Spec } from './spec';

// ─── Reports ───

/**
 * One row of the month, by payment method. `service_fees` is optional in the
 * document because only the totals rows carry it — {@link MonthlyReportTotals}
 * is the same schema with that field required.
 */
export type MonthlyReportMethod = Spec<'MonthlyReportSummary'>;

/**
 * The month's totals. Narrowed to require `service_fees`: the document marks
 * it "Present on the totals/previous_totals rows", and the reports table
 * renders it as a figure of its own.
 */
export type MonthlyReportTotals = Spec<'MonthlyReportSummary'> &
  Required<Pick<Spec<'MonthlyReportSummary'>, 'service_fees'>>;

/** One court's share of the month, ordered by what it took, highest first. */
export type MonthlyReportCourt = Spec<'MonthlyReportCourtSummary'>;

/**
 * `previous_totals` is the month before, totalled the same way. A figure on
 * its own cannot say whether a month went well; this is what turns the totals
 * into a direction. All zeroes for a month that predates the complex.
 */
export type MonthlyReport = Omit<Spec<'MonthlyReport'>, 'totals' | 'previous_totals'> & {
  totals: MonthlyReportTotals;
  previous_totals: MonthlyReportTotals;
};

export type MonthlyReportResponse = Omit<Ok<'reportingGetMonthlyReport'>, 'report'> & { report: MonthlyReport };

// ─── Async payments export (JOB-06) ───
//
// TODO(openapi): regenerate once the backend export-job PR lands. Hand-typed
// from `docs/auditoria-backend-2026-09-11/job06-export-job-design.md` §7
// (the literal OpenAPI YAML for `POST/GET …/reports/exports`), because
// `backend/internal/openapi/openapi.yaml` in this worktree still only knows
// the deprecated synchronous `GET …/reports/export`.

/** The job's state machine, mirrored from the `jobs` row (§4 of the design doc). */
export type PaymentsExportStatus = 'pending' | 'running' | 'done' | 'failed';

/** One field-level entry of an embedded {@link PaymentsExportProblem}. */
export interface PaymentsExportFieldError {
  field: string;
  message: string;
}

/**
 * The RFC 9457 Problem embedded as `PaymentsExport.error` on a `failed`
 * export — not a thrown `HTTPError`, since the status route answers `200`
 * even when the job itself failed (design doc §6b).
 */
export interface PaymentsExportProblem {
  type: string;
  title: string;
  status: number;
  detail?: string;
  instance?: string;
  request_id?: string;
  errors?: PaymentsExportFieldError[];
}

/**
 * One payments export job, as answered by the status endpoint.
 * `download_url`/`expires_at` appear only on `done`; `error` only on `failed`.
 */
export interface PaymentsExport {
  id: string;
  status: PaymentsExportStatus;
  download_url?: string;
  expires_at?: string;
  error?: PaymentsExportProblem;
}

/** `202` body from `POST …/reports/exports`. */
export interface CreatePaymentsExportResponse {
  export: {
    id: string;
    status: PaymentsExportStatus;
    status_url: string;
  };
}

/** `200` body from `GET …/reports/exports/{exportID}`. */
export interface GetPaymentsExportResponse {
  export: PaymentsExport;
}
