import { ES_AR } from '@/shared/i18n/es_AR';
import { pesosToCentavos } from '@/shared/lib/money';
import { formatPrice } from '@/shared/lib/utils';
import type { CategoryFormValues } from '../../schemas/tournamentForm.schema';

const t = ES_AR.tournaments;

/** "Zonas y cuadro · 16 parejas · $20.000": what a category is, on one line. */
export function categorySummaryLine(category: CategoryFormValues): string {
  return [
    t.format[category.format],
    t.createSteps.capacityShort(category.capacity),
    formatPrice(pesosToCentavos(category.price)),
  ].join(' · ');
}
