import { Button } from '@/shared/components/ui/button';
import { cn } from '@/shared/lib/utils';
import { SEGMENTED_TABS_CLASS } from '../lib/tournamentLayout';
import { SEGMENT_CLASS } from './segmentedTabsLayout';

interface SegmentedTabsProps<T extends string> {
  items: { value: T; label: string }[];
  active: T;
  onChange: (value: T) => void;
  ariaLabel: string;
}

/**
 * The app's segmented control: 40px toggles, the same switch the Eventos list
 * uses for Próximos and Pasados. On a phone they lay out in two columns and from
 * `sm` in one row; labels never break or scroll, whatever the width.
 */
export function SegmentedTabs<T extends string>({ items, active, onChange, ariaLabel }: SegmentedTabsProps<T>) {
  return (
    <div role="group" aria-label={ariaLabel} className={SEGMENTED_TABS_CLASS}>
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
