import { ES_AR } from '@/shared/i18n/es_AR';
import type { Court } from '@/shared/types/api.types';

const t = ES_AR;

/** "Césped sintético · Con iluminación": what the court is made of and whether it has lights; '' when neither is known. */
export function courtDetailsLine(court: Pick<Court, 'surface' | 'has_lighting'>): string {
  const parts: string[] = [];
  if (court.surface) parts.push(t.courts.surfaces[court.surface]);
  if (court.has_lighting) parts.push(t.courts.withLighting);
  return parts.join(' · ');
}
