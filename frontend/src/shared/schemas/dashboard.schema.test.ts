import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { makeBooking } from '@/test/factories';
import {
  dashboardStatsResponseSchema,
  clientInsightsResponseSchema,
  revenueChartResponseSchema,
  occupancyChartResponseSchema,
} from './dashboard.schema';

describe('dashboardStatsResponseSchema', () => {
  it('validates dashboardApi.getStats response shape', () => {
    const result = dashboardStatsResponseSchema.safeParse({
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
    });
    expect(result.success).toBe(true);
  });
});

describe('clientInsightsResponseSchema', () => {
  it('validates dashboardApi.getClientInsights response shape', () => {
    const result = clientInsightsResponseSchema.safeParse({
      clients: {
        top: [{ id: 'cl1', name: 'Juan', phone: '1155550000', booking_count: 5, total_spent: 25000 }],
        no_show_rate: 0.1,
        no_show_count: 1,
        resolved_count: 9,
        new_clients_30d: 3,
        recurring_30d: 6,
        total_active_30d: 9,
      },
    });
    expect(result.success).toBe(true);
  });
});

describe('revenueChartResponseSchema', () => {
  it('validates dashboardApi.getRevenue response shape', () => {
    const result = revenueChartResponseSchema.safeParse({ revenue: [{ date: '2026-03-01', amount: 12000 }] });
    expect(result.success).toBe(true);
  });
});

describe('occupancyChartResponseSchema', () => {
  it('validates dashboardApi.getOccupancy response shape', () => {
    const result = occupancyChartResponseSchema.safeParse({
      occupancy: [{ day_of_week: 1, hour: 20, percentage: 0.8 }],
    });
    expect(result.success).toBe(true);
  });
});
