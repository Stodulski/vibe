import type { User, UserRole } from './auth';
import type { Complex } from './complex';

// ─── Admin ───

export interface PlatformStats {
  total_users: number;
  active_users: number;
  new_users_month: number;
  total_complexes: number;
  new_complexes_month: number;
  total_courts: number;
  total_bookings: number;
  total_revenue: number;
}

export interface PlatformStatsResponse {
  stats: PlatformStats;
}

export interface AdminUserRow {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  phone: string;
  role: UserRole;
  is_active: boolean;
  email_verified: boolean;
  created_at: string;
  complex_count: number;
}

export interface AdminUsersResponse {
  users: AdminUserRow[];
  metadata: {
    next_cursor?: string;
    has_more: boolean;
    total_count?: number;
  };
}

export interface AdminUserDetailResponse {
  user: User;
  complexes: Complex[];
}

export interface AdminComplexRow {
  id: string;
  owner_id: string;
  owner_name: string;
  owner_email: string;
  name: string;
  slug: string;
  city: string;
  is_active: boolean;
  courts_count: number;
  mp_connected: boolean;
  created_at: string;
}

export interface AdminComplexesResponse {
  complexes: AdminComplexRow[];
  metadata: {
    next_cursor?: string;
    has_more: boolean;
    total_count?: number;
  };
}

export interface AdminComplexDetailResponse {
  complex: Complex;
  owner_name: string;
  owner_email: string;
  courts_count: number;
  clients_count: number;
  bookings_count: number;
  total_revenue: number;
}
