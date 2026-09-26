import { useRef, type ReactNode } from 'react';
import { Button } from '@/shared/components/ui/button';
import { useDragScroll } from '@/shared/hooks/useDragScroll';
import { cn } from '@/shared/lib/utils';

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
/**
 * Horizontal chip slider. Touch pans natively; a mouse can drag it too
 * (`useDragScroll`), and a thin brand-green bar 5px under the chips shows
 * there is more to scroll. Snapping is off while dragging so it does not
 * fight the pointer.
 */
export function ChipList({ 'aria-label': ariaLabel, children }: ChipListProps) {
  const ref = useRef<HTMLDivElement>(null);
  const { dragging, dragHandlers } = useDragScroll(ref);

  return (
    <div
      ref={ref}
      className={cn(
        'scrollbar-accent flex gap-1.5 overflow-x-auto pb-[5px] select-none',
        dragging ? 'cursor-grabbing snap-none' : 'snap-x',
      )}
      role="group"
      aria-label={ariaLabel}
      {...dragHandlers}
    >
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
