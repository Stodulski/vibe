import api, { withSignal } from '@/shared/lib/ky';
import { parseWith } from '@/shared/lib/apiParse';
import {
  dashboardStatsResponseSchema,
  revenueChartResponseSchema,
  occupancyChartResponseSchema,
  clientInsightsResponseSchema,
} from '@/shared/schemas/dashboard.schema';
import {
  monthlyReportResponseSchema,
  createPaymentsExportResponseSchema,
  getPaymentsExportResponseSchema,
} from '@/shared/schemas/reports.schema';
import type {
  DashboardStatsResponse,
  RevenueChartResponse,
  OccupancyChartResponse,
  ClientInsightsResponse,
  MonthlyReportResponse,
  CreatePaymentsExportResponse,
  GetPaymentsExportResponse,
} from '@/shared/types/api.types';

export const dashboardApi = {
  getStats: (complexId: string, signal?: AbortSignal): Promise<DashboardStatsResponse> =>
    api
      .get(`complexes/${complexId}/stats`, withSignal(signal))
      .json()
      .then(parseWith(dashboardStatsResponseSchema, 'dashboardApi.getStats')),

  getRevenue: (complexId: string, period: 'week' | 'month', signal?: AbortSignal): Promise<RevenueChartResponse> =>
    api
      .get(`complexes/${complexId}/stats/revenue`, {
        searchParams: { period },
        ...withSignal(signal),
      })
      .json()
      .then(parseWith(revenueChartResponseSchema, 'dashboardApi.getRevenue')),

  getOccupancy: (complexId: string, weeks = 4, signal?: AbortSignal): Promise<OccupancyChartResponse> =>
    api
      .get(`complexes/${complexId}/stats/occupancy`, {
        searchParams: { weeks: String(weeks) },
        ...withSignal(signal),
      })
      .json()
      .then(parseWith(occupancyChartResponseSchema, 'dashboardApi.getOccupancy')),

  getClientInsights: (complexId: string, signal?: AbortSignal): Promise<ClientInsightsResponse> =>
    api
      .get(`complexes/${complexId}/stats/clients`, withSignal(signal))
      .json()
      .then(parseWith(clientInsightsResponseSchema, 'dashboardApi.getClientInsights')),

  getMonthlyReport: (
    complexId: string,
    month: number,
    year: number,
    signal?: AbortSignal,
  ): Promise<MonthlyReportResponse> =>
    api
      .get(`complexes/${complexId}/reports/monthly`, {
        searchParams: { month: String(month), year: String(year) },
        ...withSignal(signal),
      })
      .json()
      .then(parseWith(monthlyReportResponseSchema, 'dashboardApi.getMonthlyReport')),

  /**
   * The synchronous export, deprecated by JOB-06 but kept as the fallback
   * `useReportExport` falls back to against a backend that doesn't yet know
   * the async job routes (a bare 404) or has them wired but no object
   * storage configured (501) — see `useReportExport.ts`.
   */
  exportPaymentsExcel: (complexId: string, month: number, year: number) =>
    api
      .get(`complexes/${complexId}/reports/export`, {
        searchParams: { month: String(month), year: String(year) },
        // The backend builds the workbook synchronously, so this is the one
        // call that legitimately outlives the client's 10 s default.
        timeout: 60_000,
      })
      .blob(),

  /**
   * Starts the async payments export job (JOB-06). Answers `202` with the
   * job's id and `status_url` to poll — deduplicated per complex, period and
   * calendar day, so a double click yields the same export id. `501` means
   * no object storage backend is configured; `useReportExport` reads that
   * (and a plain 404, an older backend) as "fall back to the synchronous
   * export" rather than a user-facing error.
   */
  requestPaymentsExport: (
    complexId: string,
    period: { month: number; year: number },
  ): Promise<CreatePaymentsExportResponse> =>
    api
      .post(`complexes/${complexId}/reports/exports`, { json: period })
      .json()
      .then(parseWith(createPaymentsExportResponseSchema, 'dashboardApi.requestPaymentsExport')),

  /**
   * Polls one export's status. `download_url` is a 15-minute-signed link,
   * present only once `status` is `done`; a `failed` export still answers
   * `200` with an embedded Problem in `error`. A `410` means the file has
   * passed its 24h retention.
   */
  getPaymentsExport: (complexId: string, exportId: string, signal?: AbortSignal): Promise<GetPaymentsExportResponse> =>
    api
      .get(`complexes/${complexId}/reports/exports/${exportId}`, withSignal(signal))
      .json()
      .then(parseWith(getPaymentsExportResponseSchema, 'dashboardApi.getPaymentsExport')),
};
