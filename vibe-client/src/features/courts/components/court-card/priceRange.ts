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

export function formatPriceLabel(
  min: number | null,
  max: number | null,
  formatPrice: (value: number) => string,
): string {
  if (min === null) return '—';
  if (max !== null && max !== min) return `${formatPrice(min)} - ${formatPrice(max)}`;
  return formatPrice(min);
}
