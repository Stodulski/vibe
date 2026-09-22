import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * `counted - expected`'s label and color, one surplus/shortfall/no-difference
 * rule shared by every place that shows a difference: `CashDifference`'s own
 * row and `CashSessionHistoryRow`'s compact inline figure. Kept out of
 * `CashDifference.tsx` (a component file) so exporting it doesn't break that
 * file's fast-refresh contract (react-refresh/only-export-components).
 */
export function cashDifferenceLabel(difference: number): { label: string; colorClass: string } {
  if (difference > 0) return { label: t.cash.surplus, colorClass: 'text-success-text' };
  if (difference < 0) return { label: t.cash.shortfall, colorClass: 'text-error-text' };
  return { label: t.cash.noDifference, colorClass: 'text-text-tertiary' };
}
