import { Button } from '@/shared/components/ui/button';
import { SEGMENTED_TABS_CLASS } from '../lib/tournamentLayout';

interface SegmentedTabsProps<T extends string> {
  items: { value: T; label: string }[];
  active: T;
  onChange: (value: T) => void;
  ariaLabel: string;
}

/**
 * A row of toggle buttons. When the labels do not fit (320 px, long category
 * names) the row scrolls inside its own container, never the page.
 */
export function SegmentedTabs<T extends string>({ items, active, onChange, ariaLabel }: SegmentedTabsProps<T>) {
  return (
    <div role="group" aria-label={ariaLabel} className={SEGMENTED_TABS_CLASS}>
      {items.map((item) => (
        <Button
          key={item.value}
          type="button"
          size="sm"
          variant={active === item.value ? 'default' : 'outline'}
          aria-pressed={active === item.value}
          className="shrink-0 rounded-xl px-4"
          onClick={() => {
            onChange(item.value);
          }}
        >
          {item.label}
        </Button>
      ))}
    </div>
  );
}
