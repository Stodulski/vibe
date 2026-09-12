import type { MonthlyReport } from '@/shared/types/api.types';

/**
 * What `ReportCard` is showing, as one value instead of the
 * `isLoading` + `isError` + `report?` triple it used to take: those four
 * booleans-and-a-maybe could describe eight combinations, only four of
 * which the card could render, and it sorted them out with a chain of
 * nested ternaries. Derived once, in the page, from the query result.
 */
export type ReportCardState =
  { status: 'loading' } | { status: 'error' } | { status: 'empty' } | { status: 'ready'; report: MonthlyReport };

interface MonthlyReportQuery {
  isLoading: boolean;
  isError: boolean;
  report: MonthlyReport | undefined;
}

export function toReportCardState({ isLoading, isError, report }: MonthlyReportQuery): ReportCardState {
  if (isLoading) return { status: 'loading' };
  if (isError) return { status: 'error' };
  // A month with no payments at all reads as empty, not as a report of
  // zeroes — and so does a resolved query that somehow carries no report.
  if (!report || (Object.keys(report.by_method).length === 0 && report.totals.count === 0)) {
    return { status: 'empty' };
  }
  return { status: 'ready', report };
}
