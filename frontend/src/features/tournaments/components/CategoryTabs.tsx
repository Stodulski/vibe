import { cn } from '@/shared/lib/utils';
import { CATEGORY_TAB_CLASS, CATEGORY_TABS_STRIP_CLASS } from './categoryTabsLayout';

interface CategoryTabsProps<T extends string> {
  /** A muted item belongs to something cancelled: it stays pickable but never reads as the live choice. */
  items: { value: T; label: string; muted?: boolean }[];
  active: T;
  onChange: (value: T) => void;
  ariaLabel: string;
}

/**
 * The categories as underline tabs, the same strip the Eventos tabs use. A
 * category is what the owner looks at first, so it sits above the section
 * switch of that category and never reads like it.
 */
export function CategoryTabs<T extends string>({ items, active, onChange, ariaLabel }: CategoryTabsProps<T>) {
  return (
    <nav aria-label={ariaLabel}>
      <div className={CATEGORY_TABS_STRIP_CLASS}>
        {items.map((item) => {
          const selected = active === item.value;
          return (
            <button
              key={item.value}
              type="button"
              aria-pressed={selected}
              data-cancelled={item.muted ? 'true' : undefined}
              className={cn(
                CATEGORY_TAB_CLASS,
                selected
                  ? item.muted
                    ? 'border-border-default text-text-secondary'
                    : 'border-primary-500 text-primary-400'
                  : item.muted
                    ? 'text-text-tertiary hover:text-text-secondary'
                    : 'text-text-secondary hover:text-text-primary',
              )}
              onClick={() => {
                onChange(item.value);
              }}
            >
              {item.label}
            </button>
          );
        })}
      </div>
    </nav>
  );
}
