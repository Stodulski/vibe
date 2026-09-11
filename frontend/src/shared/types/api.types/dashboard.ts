import type { Booking } from './booking';

// ─── Dashboard ───

export interface PaymentSummary {
  /**
   * Keyed by the booking's `collection_status` (backend split
   * the old payment_status enum and GetPaymentSummary groups by the money-in
   * axis). Left as `Record<string, …>` rather than
   * `Record<CollectionStatus, …>`: it is a server-built map, the client renders
   * whatever keys arrive, and a narrower type here would be a promise this
   * build cannot keep about a server that is ahead of it.
   */
  by_status: Record<string, { count: number; total: number }>;
  by_method: Record<string, number>;
}

export interface TopClient {
  id: string;
  name: string;
  phone: string;
  booking_count: number;
  total_spent: number;
}

export interface ClientInsights {
  top: TopClient[];
  no_show_rate: number;
  no_show_count: number;
  resolved_count: number;
  new_clients_30d: number;
  recurring_30d: number;
  total_active_30d: number;
}

export interface ClientInsightsResponse {
  clients: ClientInsights;
}

export interface DashboardStats {
  today_bookings: number;
  yesterday_bookings: number;
  today_revenue: number;
  yesterday_revenue: number;
  weekly_revenue: number;
  monthly_revenue: number;
  occupancy_rate: number;
  pending_bookings: number;
  total_clients: number;
  upcoming_bookings: Booking[];
  payment_summary: PaymentSummary;
}

export interface DashboardStatsResponse {
  stats: DashboardStats;
}

export interface RevenueDataPoint {
  date: string;
  amount: number;
}

export interface RevenueChartResponse {
  revenue: RevenueDataPoint[];
}

export interface OccupancyDataPoint {
  day_of_week: number;
  hour: number;
  percentage: number;
}

export interface OccupancyChartResponse {
  occupancy: OccupancyDataPoint[];
}
