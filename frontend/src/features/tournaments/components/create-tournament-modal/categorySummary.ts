import { ES_AR } from '@/shared/i18n/es_AR';
import { pesosToCentavos } from '@/shared/lib/money';
import { categoryStartLabel, categoryTitle } from '@/shared/lib/traitLabels';
import { formatPrice } from '@/shared/lib/utils';
import type { CategoryFormValues } from '../../schemas/tournamentForm.schema';

const t = ES_AR.tournaments;

/** The category's composed title in its tournament's sport: "Caballeros 8va", "Mixto Suma 15", or "Masculino" off padel. */
export function categoryFormTitle(category: CategoryFormValues, sport: string): string {
  return categoryTitle({ sport, modality: category.modality, level: category.level, suma: category.suma });
}

/** "Sáb 21 nov · 18:00 · Zonas y cuadro · 16 parejas · $20.000": when it starts, how it plays and what it costs. */
export function categorySummaryLine(category: CategoryFormValues): string {
  return [
    categoryStartLabel(category.starts_on, category.starts_at),
    t.format[category.format],
    t.createSteps.capacityShort(category.capacity),
    formatPrice(pesosToCentavos(category.price)),
  ]
    .filter((part): part is string => !!part)
    .join(' · ');
}
