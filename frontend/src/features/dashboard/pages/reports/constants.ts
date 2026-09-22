import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export const MONTH_NAMES = [
  'Enero',
  'Febrero',
  'Marzo',
  'Abril',
  'Mayo',
  'Junio',
  'Julio',
  'Agosto',
  'Septiembre',
  'Octubre',
  'Noviembre',
  'Diciembre',
] as const;

export const METHOD_LABELS: Record<string, string> = t.bookings.paymentMethods;
