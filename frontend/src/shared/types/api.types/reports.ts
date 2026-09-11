// ─── Reports ───

export interface MonthlyReportMethod {
  count: number;
  total: number;
  refunded: number;
  net: number;
}

export interface MonthlyReportTotals {
  count: number;
  total: number;
  service_fees: number;
  refunded: number;
  net: number;
}

/** One court's share of the month, ordered by what it took, highest first. */
export interface MonthlyReportCourt {
  court_id: string;
  /** Empty when the court has since been deleted; the money still counts. */
  court_name: string;
  count: number;
  total: number;
  refunded: number;
  net: number;
}

export interface MonthlyReport {
  month: number;
  year: number;
  by_method: Record<string, MonthlyReportMethod>;
  by_court: MonthlyReportCourt[];
  totals: MonthlyReportTotals;
  /**
   * The month before, totalled the same way. A figure on its own cannot say
   * whether a month went well; this is what turns the totals into a direction.
   * All zeroes for a month that predates the complex.
   */
  previous_totals: MonthlyReportTotals;
}

export interface MonthlyReportResponse {
  report: MonthlyReport;
}
