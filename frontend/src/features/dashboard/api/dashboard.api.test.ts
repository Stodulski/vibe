import { describe, it, expect, vi } from 'vitest';
// @vitest-environment node
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { dashboardApi } from './dashboard.api';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { makeBooking } from '@/test/factories';

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

const STATS_RESPONSE = {
  stats: {
    today_bookings: 3,
    yesterday_bookings: 2,
    today_revenue: 15000,
    yesterday_revenue: 10000,
    weekly_revenue: 80000,
    monthly_revenue: 300000,
    occupancy_rate: 0.6,
    pending_bookings: 1,
    total_clients: 50,
    upcoming_bookings: [makeBooking()],
    payment_summary: {
      by_status: { unpaid: { count: 1, total: 5000 } },
      by_method: { cash: 2 },
    },
  },
};

describe('dashboardApi — stats and revenue', () => {
  it('getStats parses a complete DashboardStatsResponse', async () => {
    server.use(http.get('*/complexes/:complexId/stats', () => HttpResponse.json(STATS_RESPONSE)));

    const result = await dashboardApi.getStats('c1');

    expect(result).toEqual(STATS_RESPONSE);
  });

  it('getStats rejects with ApiResponseError when the response body does not match the schema', async () => {
    server.use(http.get('*/complexes/:complexId/stats', () => HttpResponse.json({ stats: { today_bookings: 3 } })));

    await expect(dashboardApi.getStats('c1')).rejects.toThrow(ApiResponseError);
  });

  it('getRevenue parses a RevenueChartResponse and sends the period as a search param', async () => {
    let receivedPeriod: string | null = null;
    server.use(
      http.get('*/complexes/:complexId/stats/revenue', ({ request }) => {
        receivedPeriod = new URL(request.url).searchParams.get('period');
        return HttpResponse.json({ revenue: [{ date: '2026-03-01', amount: 12000 }] });
      }),
    );

    const result = await dashboardApi.getRevenue('c1', 'week');

    expect(receivedPeriod).toBe('week');
    expect(result).toEqual({ revenue: [{ date: '2026-03-01', amount: 12000 }] });
  });

  it('getOccupancy parses an OccupancyChartResponse', async () => {
    server.use(
      http.get('*/complexes/:complexId/stats/occupancy', () =>
        HttpResponse.json({ occupancy: [{ day_of_week: 1, hour: 20, percentage: 0.8 }] }),
      ),
    );

    const result = await dashboardApi.getOccupancy('c1', 4);

    expect(result).toEqual({ occupancy: [{ day_of_week: 1, hour: 20, percentage: 0.8 }] });
  });
});

describe('dashboardApi — client insights, monthly report and export', () => {
  it('getClientInsights parses a ClientInsightsResponse', async () => {
    const response = {
      clients: {
        top: [{ id: 'cl1', name: 'Juan', phone: '1155550000', booking_count: 5, total_spent: 25000 }],
        no_show_rate: 0.1,
        no_show_count: 1,
        resolved_count: 9,
        new_clients_30d: 3,
        recurring_30d: 6,
        total_active_30d: 9,
      },
    };
    server.use(http.get('*/complexes/:complexId/stats/clients', () => HttpResponse.json(response)));

    await expect(dashboardApi.getClientInsights('c1')).resolves.toEqual(response);
  });

  it('getMonthlyReport parses a MonthlyReportResponse', async () => {
    const totals = { count: 10, total: 100000, service_fees: 5000, refunded: 2000, net: 93000 };
    const response = {
      report: {
        month: 3,
        year: 2026,
        by_method: { cash: { count: 5, total: 50000, refunded: 0, net: 50000 } },
        by_court: [
          {
            court_id: 'ct1',
            court_name: 'Cancha 1',
            count: 10,
            total: 100000,
            refunded: 2000,
            net: 93000,
          },
        ],
        totals,
        previous_totals: totals,
      },
    };
    server.use(http.get('*/complexes/:complexId/reports/monthly', () => HttpResponse.json(response)));

    await expect(dashboardApi.getMonthlyReport('c1', 3, 2026)).resolves.toEqual(response);
  });

  it('exportPaymentsExcel returns a blob without JSON parsing', async () => {
    const bytes = new TextEncoder().encode('x');
    server.use(http.get('*/complexes/:complexId/reports/export', () => HttpResponse.arrayBuffer(bytes.buffer)));

    const blob = await dashboardApi.exportPaymentsExcel('c1', 3, 2026);

    expect(blob).toBeInstanceOf(Blob);
    expect(await blob.text()).toBe('x');
  });
});
