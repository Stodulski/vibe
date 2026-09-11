import { z } from 'zod';
import type {
  MonthlyReportMethod,
  MonthlyReportTotals,
  MonthlyReportCourt,
  MonthlyReport,
  MonthlyReportResponse,
} from '@/shared/types/api.types';

// ─── Reports ───

export const monthlyReportMethodSchema = z
  .object({
    count: z.number(),
    total: z.number(),
    refunded: z.number(),
    net: z.number(),
  })
  .loose() satisfies z.ZodType<MonthlyReportMethod>;

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

export const monthlyReportSchema = z
  .object({
    month: z.number(),
    year: z.number(),
    by_method: z.record(z.string(), monthlyReportMethodSchema),
    by_court: z.array(monthlyReportCourtSchema),
    totals: monthlyReportTotalsSchema,
    previous_totals: monthlyReportTotalsSchema,
  })
  .loose() satisfies z.ZodType<MonthlyReport>;

export const monthlyReportResponseSchema = z
  .object({
    report: monthlyReportSchema,
  })
  .loose() satisfies z.ZodType<MonthlyReportResponse>;
