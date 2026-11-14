import { pesosToCentavos } from '@/shared/lib/money';
import type { CreateBookingDto } from '../../schemas/booking.schema';
import type { SeriesEnd } from './useSeriesOptions';
import type { CreateBookingSeriesRequest } from '@/shared/types/api.types';

/**
 * The create-series body from the same form values an ordinary booking uses.
 * Payment fields are left out on purpose: every occurrence is charged like a
 * normal booking, at the court. `price` (manual, pesos in the form) goes to
 * centavos and applies to every occurrence; without it each one is priced when
 * it is generated.
 */
export function buildSeriesPayload(data: CreateBookingDto, end: SeriesEnd): CreateBookingSeriesRequest {
  return {
    court_id: data.court_id,
    first_date: data.date,
    start_time: data.start_time,
    duration_minutes: data.duration_minutes,
    client_phone: data.client_phone,
    client_first_name: data.client_first_name,
    client_last_name: data.client_last_name,
    ...(data.client_email ? { client_email: data.client_email } : {}),
    ...(data.notes ? { notes: data.notes } : {}),
    ...(typeof data.price === 'number' ? { price: pesosToCentavos(data.price) } : {}),
    ...end,
  };
}
