import { useId } from 'react';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { cn } from '@/shared/lib/utils';
import { CATEGORY_TAB_CLASS, CATEGORY_TAB_LABEL_CLASS, CATEGORY_TABS_STRIP_CLASS } from './categoryTabsLayout';

/** Past this many categories the choice is a select: a row of tabs would not fit a phone. */
const SELECT_FROM = 4;

interface CategoryTabsProps<T extends string> {
  /** A muted item belongs to something cancelled: it stays pickable but never reads as the live choice. */
  items: { value: T; label: string; muted?: boolean }[];
  active: T;
  onChange: (value: T) => void;
  ariaLabel: string;
}

/**
 * The categories as underline tabs, the same strip the Eventos tabs use, wrapping
 * onto more lines when the names do not fit. From four categories they become a
 * select with the same label. A category is what the owner looks at first, so it
 * sits above the section switch of that category and never reads like it.
 */
export function CategoryTabs<T extends string>({ items, active, onChange, ariaLabel }: CategoryTabsProps<T>) {
  if (items.length >= SELECT_FROM) {
    return <CategorySelect items={items} active={active} onChange={onChange} ariaLabel={ariaLabel} />;
  }
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
              title={item.label}
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
              <span className={CATEGORY_TAB_LABEL_CLASS}>{item.label}</span>
            </button>
          );
        })}
      </div>
    </nav>
  );
}

function CategorySelect<T extends string>({ items, active, onChange, ariaLabel }: CategoryTabsProps<T>) {
  const id = useId();
  return (
    <div className="min-w-0 space-y-1.5">
      <Label htmlFor={id}>{ariaLabel}</Label>
      <Select
        value={active}
        onValueChange={(value) => {
          const next = items.find((item) => item.value === value);
          if (next) onChange(next.value);
        }}
      >
        <SelectTrigger id={id} className="w-full min-w-0 [&>span]:truncate">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
