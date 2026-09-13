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
// The backend's export-job PR (#56) landed and `openapi.yaml` now declares
// both routes — these were hand-typed from the design doc's literal YAML
// until this regeneration; every type below now derives from the generated
// document instead of restating it.

/**
 * The RFC 9457 Problem embedded as `PaymentsExport.error` on a `failed`
 * export — not a thrown `HTTPError`, since the status route answers `200`
 * even when the job itself failed. Same generated `Problem` schema every
 * thrown `HTTPError` carries (see `src/shared/lib/ApiError.ts`), just never
 * `zod`-validated there because it arrives already-thrown, not embedded in a
 * 2xx body the way this one is.
 */
export type PaymentsExportProblem = Spec<'Problem'>;

/**
 * One payments export job, as answered by the status endpoint.
 * `download_url`/`expires_at` appear only on `done`; `error` only on `failed`.
 */
export type PaymentsExport = Spec<'PaymentsExport'>;

/** `202` body from `POST …/reports/exports`. */
export type CreatePaymentsExportResponse = Ok<'reportingCreatePaymentsExport'>;

/** `200` body from `GET …/reports/exports/{exportID}`. */
export type GetPaymentsExportResponse = Ok<'reportingGetPaymentsExport'>;
