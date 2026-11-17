import type { ReactNode } from 'react';
import { cn } from '@/shared/lib/utils';
import { LabeledSelect } from './LabeledSelect';
import {
  CATEGORY_SWITCH_ROW_CLASS,
  CATEGORY_TAB_CLASS,
  CATEGORY_TAB_LABEL_CLASS,
  CATEGORY_TABS_STRIP_CLASS,
} from './categoryTabsLayout';

/** Past this many categories the choice is a select: a row of tabs would not fit a phone. */
const SELECT_FROM = 4;

interface CategoryTabsProps<T extends string> {
  /** A muted item belongs to something cancelled: it stays pickable but never reads as the live choice. */
  items: { value: T; label: string; muted?: boolean }[];
  active: T;
  onChange: (value: T) => void;
  ariaLabel: string;
  /**
   * The section switch of the chosen category. As a select it shares the category
   * select's row, side by side; under the tabs it sits on a line of its own.
   */
  beside?: ReactNode;
}

/**
 * The categories as underline tabs, the same strip the Eventos tabs use, wrapping
 * onto more lines when the names do not fit. From four categories they become a
 * select with the same label. A category is what the owner looks at first, so it
 * sits above the section switch of that category and never reads like it.
 */
export function CategoryTabs<T extends string>({ items, active, onChange, ariaLabel, beside }: CategoryTabsProps<T>) {
  if (items.length >= SELECT_FROM) {
    const select = <LabeledSelect label={ariaLabel} items={items} active={active} onChange={onChange} />;
    return beside ? (
      <div className={CATEGORY_SWITCH_ROW_CLASS}>
        {select}
        {beside}
      </div>
    ) : (
      select
    );
  }
  return (
    <>
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
      {beside}
    </>
  );
}
