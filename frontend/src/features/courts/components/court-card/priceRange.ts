import type { CourtPrice } from '@/shared/types/api.types';

const WEEKDAY_KEYS = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday'];
const WEEKEND_KEYS = ['saturday', 'sunday'];

export interface PriceRange {
  weekdayMin: number | null;
  weekdayMax: number | null;
  weekendMin: number | null;
  weekendMax: number | null;
  hasPrices: boolean;
}

export function getPriceRange(prices: CourtPrice[]): PriceRange {
  const wd = prices.filter((p) => WEEKDAY_KEYS.includes(p.day_type));
  const we = prices.filter((p) => WEEKEND_KEYS.includes(p.day_type));
  return {
    weekdayMin: wd.length > 0 ? Math.min(...wd.map((p) => p.price)) : null,
    weekdayMax: wd.length > 0 ? Math.max(...wd.map((p) => p.price)) : null,
    weekendMin: we.length > 0 ? Math.min(...we.map((p) => p.price)) : null,
    weekendMax: we.length > 0 ? Math.max(...we.map((p) => p.price)) : null,
    hasPrices: prices.length > 0,
  };
}

export interface OverallPriceRange {
  min: number | null;
  max: number | null;
  hasPrices: boolean;
}

/**
 * One range across every band, regardless of day — what the `xl` table's
 * single "Precio" column shows instead of the card's separate weekday/weekend
 * lines. A dense row has room for one figure, not four.
 */
export function getOverallPriceRange(prices: CourtPrice[]): OverallPriceRange {
  if (prices.length === 0) return { min: null, max: null, hasPrices: false };
  const values = prices.map((p) => p.price);
  return { min: Math.min(...values), max: Math.max(...values), hasPrices: true };
}

export function formatPriceLabel(
  min: number | null,
  max: number | null,
  formatPrice: (value: number) => string,
): string {
  if (min === null) return '—';
  if (max !== null && max !== min) return `${formatPrice(min)} - ${formatPrice(max)}`;
  return formatPrice(min);
}
