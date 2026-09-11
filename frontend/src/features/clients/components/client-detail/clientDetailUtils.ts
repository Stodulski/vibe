import type { Client } from '@/shared/types/api.types';

export function getClientAttendance(client: Client): number {
  return client.total_bookings > 0
    ? Math.round(((client.total_bookings - client.no_shows) / client.total_bookings) * 100)
    : 100;
}
