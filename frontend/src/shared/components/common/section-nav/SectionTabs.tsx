import { cn } from '@/shared/lib/utils';
import type { SectionNavItem } from './SectionNav';

interface SectionTabsProps<V extends string> {
  items: readonly SectionNavItem<V>[];
  active: V;
  onChange: (value: V) => void;
}

/**
 * The same sections on a phone.
 *
 * The tabs share the width evenly instead of scrolling: a strip that overflows
 * gives no sign that the sections past its edge exist at all, and the settings
 * version of this used to hide two of its five that way.
 *
 * No band and no bottom rule — a full-width line under the strip read as the
 * top edge of a card, which is exactly what these are not. The active tab's
 * own underline marks the selection. 48px tall to clear the minimum target.
 */
export function SectionTabs<V extends string>({ items, active, onChange }: SectionTabsProps<V>) {
  return (
    <div className="-mx-4 px-4 lg:hidden">
      <div className="flex">
        {items.map((item) => {
          const isActive = active === item.value;
          return (
            <button
              key={item.value}
              type="button"
              aria-pressed={isActive}
              onClick={() => {
                onChange(item.value);
              }}
              className={cn(
                'flex h-12 flex-1 items-center justify-center gap-2 border-b-2 px-2 text-sm font-medium transition-colors',
                isActive
                  ? 'border-primary-500 text-primary-400'
                  : 'border-transparent text-text-secondary active:scale-[0.97]',
              )}
            >
              <item.icon className="size-4 shrink-0" />
              <span className="truncate">{item.label}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
