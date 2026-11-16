import { Button } from '@/shared/components/ui/button';
import { cn } from '@/shared/lib/utils';
import { SEGMENTED_TABS_CLASS } from '../lib/tournamentLayout';

interface SegmentedTabsProps<T extends string> {
  /** A muted item belongs to something cancelled: it stays pickable but never reads as the live, green choice. */
  items: { value: T; label: string; muted?: boolean }[];
  active: T;
  onChange: (value: T) => void;
  ariaLabel: string;
  /** Wrap onto more lines instead of scrolling sideways: for a few choices inside a narrow dialog. */
  wrap?: boolean;
}

/**
 * A row of toggle buttons. When the labels do not fit (320 px, long category
 * names) the row scrolls inside its own container, never the page.
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
      {items.map((item) => (
        <Button
          key={item.value}
          type="button"
          size="sm"
          variant={active === item.value && !item.muted ? 'default' : 'outline'}
          aria-pressed={active === item.value}
          {...(item.muted ? { 'data-cancelled': 'true' } : {})}
          className={cn(
            'shrink-0 rounded-xl px-4',
            item.muted && 'text-text-tertiary',
            item.muted && active === item.value && 'border-border-default bg-bg-subtle',
          )}
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
