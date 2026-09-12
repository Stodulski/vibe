import { z } from 'zod';
import { exact } from '@/shared/lib/apiParse';
import type {
  MonthlyReportMethod,
  MonthlyReportTotals,
  MonthlyReportCourt,
  MonthlyReport,
  MonthlyReportResponse,
} from '@/shared/types/api.types';

// ─── Reports ───

/** The same `MonthlyReportSummary` the totals use, minus the `service_fees` only the totals rows carry. */
export const monthlyReportMethodSchema = exact<MonthlyReportMethod>(
  z
    .object({
      count: z.number(),
      total: z.number(),
      service_fees: z.number().optional(),
      refunded: z.number(),
      net: z.number(),
    })
    .loose(),
);

export const monthlyReportTotalsSchema = z
  .object({
    count: z.number(),
    total: z.number(),
    service_fees: z.number(),
    refunded: z.number(),
    net: z.number(),
  })
  .loose() satisfies z.ZodType<MonthlyReportTotals>;

export const monthlyReportCourtSchema = z
  .object({
    court_id: z.string(),
    court_name: z.string(),
    count: z.number(),
    total: z.number(),
    refunded: z.number(),
    net: z.number(),
  })
  .loose() satisfies z.ZodType<MonthlyReportCourt>;

export const monthlyReportSchema = exact<MonthlyReport>(
  z
    .object({
      month: z.number(),
      year: z.number(),
      by_method: z.record(z.string(), monthlyReportMethodSchema),
      by_court: z.array(monthlyReportCourtSchema),
      totals: monthlyReportTotalsSchema,
      previous_totals: monthlyReportTotalsSchema,
    })
    .loose(),
);

export const monthlyReportResponseSchema = exact<MonthlyReportResponse>(
  z
    .object({
      report: monthlyReportSchema,
    })
    .loose(),
);
