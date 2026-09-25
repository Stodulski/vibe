import type { Booking } from './booking';
import type { Ok, Spec } from './spec';

// ─── Dashboard ───

/**
 * `by_status` is keyed by the booking's `collection_status` (the money-in
 * axis of the old `payment_status`) and `by_method` by payment method, both
 * as open string maps: they are server-built, the client renders whatever
 * keys arrive, and `openapi.yaml` types them the same way for the same
 * reason — a narrower key type here would be a promise this build cannot
 * keep about a server that is ahead of it.
 */
export type PaymentSummary = Spec<'PaymentSummary'>;

/**
 * The dashboard's "Hoy" card money figures — everything that entered today,
 * on the Argentina calendar day, combining booking payments with cash-till
 * movements (bar sales, other manual income, expenses). See the OpenAPI
 * description on DashboardStats.today_money for how it differs from
 * `today_revenue` (bookings only) and from a cash session's shift totals.
 * `by_method` is an open string map for the same reason PaymentSummary's
 * maps are: server-built keys, rendered whatever arrives.
 */
export type DayMoneyTotals = Spec<'DayMoneyTotals'>;

export type TopClient = Spec<'TopClient'>;

export type ClientInsights = Spec<'ClientInsights'>;

export type ClientInsightsResponse = Ok<'reportingGetClientInsights'>;

export type DashboardStats = Omit<Spec<'DashboardStats'>, 'upcoming_bookings'> & { upcoming_bookings: Booking[] };

export type DashboardStatsResponse = Omit<Ok<'reportingGetDashboardStats'>, 'stats'> & { stats: DashboardStats };

export type RevenueDataPoint = Spec<'RevenueDataPoint'>;

export type RevenueChartResponse = Ok<'reportingGetRevenueChart'>;

export type OccupancyDataPoint = Spec<'OccupancyPoint'>;

export type OccupancyChartResponse = Ok<'reportingGetOccupancyChart'>;
