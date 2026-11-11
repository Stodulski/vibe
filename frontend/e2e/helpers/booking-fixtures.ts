import type { ApiBooking, ApiHelper } from './api.helper';

/**
 * Books `courtId` on `date` at the first free whole hour from 08:00 to 21:00,
 * one hour long. The shared complex keeps every earlier run's bookings, so a
 * fixed hour would collide on a rerun; a taken hour moves on to the next.
 */
export async function bookFirstFreeHour(
  api: ApiHelper,
  complexId: string,
  courtId: string,
  date: string,
  client: { firstName: string; lastName: string },
): Promise<ApiBooking> {
  for (let hour = 8; hour <= 21; hour++) {
    try {
      return await api.createBooking(complexId, courtId, {
        date,
        start_time: `${String(hour).padStart(2, '0')}:00`,
        client_first_name: client.firstName,
        client_last_name: client.lastName,
        client_phone: `+54911${String(Date.now()).slice(-8)}`,
      });
    } catch {
      // Taken by an earlier run; try the next hour.
    }
  }
  throw new Error(`bookFirstFreeHour: no free hour left on ${date}`);
}
