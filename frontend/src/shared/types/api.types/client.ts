import type { Booking } from './booking';

// ─── Client ───

export interface Client {
  id: string;
  complex_id: string;
  first_name: string;
  last_name: string;
  phone: string;
  email?: string;
  notes?: string;
  is_blocked: boolean;
  total_bookings: number;
  no_shows: number;
  created_at: string;
  updated_at: string;
}

export interface ClientsListResponse {
  clients: Client[];
  metadata: {
    next_cursor?: string;
    has_more: boolean;
    total_count?: number;
  };
}

export interface ClientDetailResponse {
  client: Client;
  recent_bookings: Booking[];
}

export interface UpdateClientRequest {
  notes?: string;
  is_blocked?: boolean;
}
