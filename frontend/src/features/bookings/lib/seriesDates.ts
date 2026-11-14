import { format } from 'date-fns/format';
import { parseISO } from 'date-fns/parseISO';
import { es } from 'date-fns/locale/es';

/** A `YYYY-MM-DD` date as "mar 18 nov". `parseISO` reads a date-only string as local midnight, so no day shifts. */
export function formatDateOnly(date: string): string {
  return format(parseISO(date), 'EEE d MMM', { locale: es });
}
