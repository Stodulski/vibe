import { format } from 'date-fns/format';
import { parseISO } from 'date-fns/parseISO';
import { es } from 'date-fns/locale/es';

/** The selected day as "lun 7 dic", for a header too narrow (320px) for "lunes 7 de diciembre". */
export function shortDayLabel(date: string): string {
  return format(parseISO(date), 'EEE d MMM', { locale: es }).replace(/\./g, '');
}
