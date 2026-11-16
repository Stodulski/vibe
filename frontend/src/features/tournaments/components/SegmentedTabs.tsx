import { Button } from '@/shared/components/ui/button';
import { cn } from '@/shared/lib/utils';
import { SEGMENTED_TABS_CLASS } from '../lib/tournamentLayout';
import { SEGMENT_CLASS } from './segmentedTabsLayout';

interface SegmentedTabsProps<T extends string> {
  items: { value: T; label: string }[];
  active: T;
  onChange: (value: T) => void;
  ariaLabel: string;
  /** Wrap onto more lines instead of scrolling sideways: for a few choices inside a narrow dialog. */
  wrap?: boolean;
}

/**
 * The app's segmented control: equal 40px toggles in one row, the same switch
 * the Eventos list uses for Próximos and Pasados. When the labels do not fit
 * (320 px) the row scrolls inside its own container, never the page.
 */
export function SegmentedTabs<T extends string>({
  items,
  active,
  onChange,
  ariaLabel,
  wrap = false,
}: SegmentedTabsProps<T>) {
  return (
    <div role="group" aria-label={ariaLabel} className={cn(SEGMENTED_TABS_CLASS, wrap && 'flex-wrap overflow-visible')}>
      {items.map((item) => {
        const selected = active === item.value;
        return (
          <Button
            key={item.value}
            type="button"
            variant={selected ? 'default' : 'outline'}
            aria-pressed={selected}
            className={cn(SEGMENT_CLASS, selected && 'border-transparent')}
            onClick={() => {
              onChange(item.value);
            }}
          >
            {item.label}
          </Button>
        );
      })}
    </div>
  );
}
