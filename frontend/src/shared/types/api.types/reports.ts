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
