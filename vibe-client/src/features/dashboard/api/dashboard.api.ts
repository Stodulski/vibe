import api, { withSignal } from '@/shared/lib/ky';
import { parseWith } from '@/shared/lib/apiParse';
import {
  dashboardStatsResponseSchema,
  revenueChartResponseSchema,
  occupancyChartResponseSchema,
  clientInsightsResponseSchema,
} from '@/shared/schemas/dashboard.schema';
import { monthlyReportResponseSchema } from '@/shared/schemas/reports.schema';
import type {
  DashboardStatsResponse,
  RevenueChartResponse,
  OccupancyChartResponse,
  ClientInsightsResponse,
  MonthlyReportResponse,
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

  exportPaymentsExcel: (complexId: string, month: number, year: number) =>
    api
      .get(`complexes/${complexId}/reports/export`, {
        searchParams: { month: String(month), year: String(year) },
      })
      .blob(),
};
