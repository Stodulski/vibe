import { describe, it, expect } from 'vitest';
import type { MonthlyReport } from '@/shared/types/api.types';
import { toReportCardState } from './reportCardState';

const emptyTotals = { count: 0, total: 0, service_fees: 0, refunded: 0, net: 0 };

const report: MonthlyReport = {
  month: 3,
  year: 2026,
  by_court: [],
  previous_totals: emptyTotals,
  by_method: { cash: { count: 5, total: 200000, refunded: 0, net: 200000 } },
  totals: { count: 5, total: 200000, service_fees: 0, refunded: 0, net: 200000 },
};

const emptyReport: MonthlyReport = { ...report, by_method: {}, totals: emptyTotals };

describe('toReportCardState', () => {
  it('reports loading before anything else, even with a stale report in hand', () => {
    expect(toReportCardState({ isLoading: true, isError: false, report })).toEqual({ status: 'loading' });
  });

  it('reports the error over a stale report', () => {
    expect(toReportCardState({ isLoading: false, isError: true, report })).toEqual({ status: 'error' });
  });

  it('reports empty for a month with no payments at all', () => {
    expect(toReportCardState({ isLoading: false, isError: false, report: emptyReport })).toEqual({ status: 'empty' });
  });

  it('reports empty when a resolved query carries no report', () => {
    expect(toReportCardState({ isLoading: false, isError: false, report: undefined })).toEqual({ status: 'empty' });
  });

  it('carries the report in the ready state', () => {
    expect(toReportCardState({ isLoading: false, isError: false, report })).toEqual({ status: 'ready', report });
  });
});
