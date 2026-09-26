import type { ReactNode } from 'react';
import { Button } from '@/shared/components/ui/button';

interface ChipListProps {
  'aria-label': string;
  children: ReactNode;
}

/**
 * Horizontal scroll-snap row of pressable chips — the pattern `SellCategoryChips`
 * (Vender's category filter) introduced, extracted here so
 * `ProductCategoryField`'s category picker (`odd/tasks/product-category-chips.md`
 * T1) can look and behave the same without duplicating the scroller/chip
 * styling. Selection semantics stay with each caller: Vender's chips are a
 * single-select filter with an "all" option, the product picker's chips
 * toggle a text input, and the two don't share enough behavior to merge.
 */
export function ChipList({ 'aria-label': ariaLabel, children }: ChipListProps) {
  return (
    <div className="flex snap-x scrollbar-none gap-1.5 overflow-x-auto pb-1" role="group" aria-label={ariaLabel}>
      {children}
    </div>
  );
}

interface ChipProps {
  selected: boolean;
  onClick: () => void;
  children: ReactNode;
}

/** One chip inside a {@link ChipList}. */
export function Chip({ selected, onClick, children }: ChipProps) {
  return (
    <Button
      type="button"
      size="sm"
      className="shrink-0 snap-start"
      variant={selected ? 'default' : 'outline'}
      aria-pressed={selected}
      onClick={onClick}
    >
      {children}
    </Button>
  );
}
