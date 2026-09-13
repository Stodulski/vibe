import { z } from 'zod';
import { exact } from '@/shared/lib/apiParse';
import type {
  MonthlyReportMethod,
  MonthlyReportTotals,
  MonthlyReportCourt,
  MonthlyReport,
  MonthlyReportResponse,
  PaymentsExportFieldError,
  PaymentsExportProblem,
  PaymentsExport,
  CreatePaymentsExportResponse,
  GetPaymentsExportResponse,
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

// ─── Async payments export (JOB-06) ───
//
// TODO(openapi): regenerate once the backend export-job PR lands — see the
// same note in `src/shared/types/api.types/reports.ts`.

export const paymentsExportStatusSchema = z.enum(['pending', 'running', 'done', 'failed']);

const paymentsExportFieldErrorSchema = z
  .object({
    field: z.string(),
    message: z.string(),
  })
  .loose() satisfies z.ZodType<PaymentsExportFieldError>;

/** The embedded Problem on a `failed` export (design doc §6b). Not the thrown-`HTTPError` `Problem` shape — this one arrives inside a `200` body. */
export const paymentsExportProblemSchema = exact<PaymentsExportProblem>(
  z
    .object({
      type: z.string(),
      title: z.string(),
      status: z.number(),
      detail: z.string().optional(),
      instance: z.string().optional(),
      request_id: z.string().optional(),
      errors: z.array(paymentsExportFieldErrorSchema).optional(),
    })
    .loose(),
);

export const paymentsExportSchema = exact<PaymentsExport>(
  z
    .object({
      id: z.string(),
      status: paymentsExportStatusSchema,
      download_url: z.string().optional(),
      expires_at: z.string().optional(),
      error: paymentsExportProblemSchema.optional(),
    })
    .loose(),
);

export const createPaymentsExportResponseSchema = exact<CreatePaymentsExportResponse>(
  z
    .object({
      export: z
        .object({
          id: z.string(),
          status: paymentsExportStatusSchema,
          status_url: z.string(),
        })
        .loose(),
    })
    .loose(),
);

export const getPaymentsExportResponseSchema = exact<GetPaymentsExportResponse>(
  z
    .object({
      export: paymentsExportSchema,
    })
    .loose(),
);
