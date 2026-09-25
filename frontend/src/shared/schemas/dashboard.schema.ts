import { z } from 'zod';
import type {
  PaymentSummary,
  TopClient,
  ClientInsights,
  ClientInsightsResponse,
  DashboardStats,
  DashboardStatsResponse,
  DayMoneyTotals,
  RevenueDataPoint,
  RevenueChartResponse,
  OccupancyDataPoint,
  OccupancyChartResponse,
} from '@/shared/types/api.types';
import { bookingSchema } from './booking.schema';

// ─── Dashboard ───

/**
 * `by_status`/`by_method` stay `z.record(z.string(), …)`, matching the
 * handwritten type's own `Record<string, …>` — the server is free to add a
 * new `collection_status` or payment method key the client hasn't been
 * updated to expect, and the type is deliberately not narrowed to
 * `CollectionStatus` for that reason (see the type's own doc comment).
 */
const paymentSummarySchema = z
  .object({
    by_status: z.record(z.string(), z.object({ count: z.number(), total: z.number() }).loose()),
    by_method: z.record(z.string(), z.number()),
  })
  .loose() satisfies z.ZodType<PaymentSummary>;

const topClientSchema = z
  .object({
    id: z.string(),
    name: z.string(),
    phone: z.string(),
    booking_count: z.number(),
    total_spent: z.number(),
  })
  .loose() satisfies z.ZodType<TopClient>;

const clientInsightsSchema = z
  .object({
    top: z.array(topClientSchema),
    no_show_rate: z.number(),
    no_show_count: z.number(),
    resolved_count: z.number(),
    new_clients_30d: z.number(),
    recurring_30d: z.number(),
    total_active_30d: z.number(),
  })
  .loose() satisfies z.ZodType<ClientInsights>;

export const clientInsightsResponseSchema = z
  .object({
    clients: clientInsightsSchema,
  })
  .loose() satisfies z.ZodType<ClientInsightsResponse>;

// `by_method` stays `z.record(z.string(), …)` for the same reason
// paymentSummarySchema's own does — server-built keys, rendered whatever
// arrives (see DayMoneyTotals's own doc comment).
const dayMoneyTotalsSchema = z
  .object({
    bookings: z.number(),
    bar_sales: z.number(),
    other_income: z.number(),
    expenses: z.number(),
    total_income: z.number(),
    by_method: z.record(z.string(), z.number()),
  })
  .loose() satisfies z.ZodType<DayMoneyTotals>;

const dashboardStatsSchema = z
  .object({
    today_bookings: z.number(),
    yesterday_bookings: z.number(),
    today_revenue: z.number(),
    yesterday_revenue: z.number(),
    weekly_revenue: z.number(),
    monthly_revenue: z.number(),
    occupancy_rate: z.number(),
    pending_bookings: z.number(),
    total_clients: z.number(),
    upcoming_bookings: z.array(bookingSchema),
    payment_summary: paymentSummarySchema,
    today_money: dayMoneyTotalsSchema,
  })
  .loose() satisfies z.ZodType<DashboardStats>;

export const dashboardStatsResponseSchema = z
  .object({
    stats: dashboardStatsSchema,
  })
  .loose() satisfies z.ZodType<DashboardStatsResponse>;

const revenueDataPointSchema = z
  .object({
    date: z.string(),
    amount: z.number(),
  })
  .loose() satisfies z.ZodType<RevenueDataPoint>;

export const revenueChartResponseSchema = z
  .object({
    revenue: z.array(revenueDataPointSchema),
  })
  .loose() satisfies z.ZodType<RevenueChartResponse>;

const occupancyDataPointSchema = z
  .object({
    day_of_week: z.number(),
    hour: z.number(),
    percentage: z.number(),
  })
  .loose() satisfies z.ZodType<OccupancyDataPoint>;

export const occupancyChartResponseSchema = z
  .object({
    occupancy: z.array(occupancyDataPointSchema),
  })
  .loose() satisfies z.ZodType<OccupancyChartResponse>;
