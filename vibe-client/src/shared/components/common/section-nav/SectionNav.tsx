import type { LucideIcon } from 'lucide-react';
import { cn } from '@/shared/lib/utils';

export interface SectionNavItem<V extends string> {
  value: V;
  label: string;
  icon: LucideIcon;
}

interface SectionNavProps<V extends string> {
  items: readonly SectionNavItem<V>[];
  active: V;
  onChange: (value: V) => void;
}

/**
 * The section list beside a settings-style screen, on a wide viewport.
 *
 * Shared because settings and profile had one each, byte-for-byte the same,
 * and they had already started to drift — the settings copy lost its truncated
 * descriptions and gained 48px rows while the profile copy kept both defects.
 *
 * Labels only. Each row used to carry the section's description as a second
 * line, which the panel then repeated a few hundred pixels to the right — and
 * the nav was too narrow to finish the sentence, so it read "Contraseña y
 * acceso a t...". One place says it, and it is the place with room.
 */
export function SectionNav<V extends string>({ items, active, onChange }: SectionNavProps<V>) {
  return (
    <nav className="hidden w-52 shrink-0 lg:block">
      {/* 5.5rem = the masthead's 64px plus 24px of air. It stuck at `top-6`,
          which is 24px from the VIEWPORT — and the masthead is `fixed` and
          64px tall, so the nav slid underneath it on scroll. Sticky offsets
          are measured against the viewport, not against the padded container
          the nav lives in. */}
      <div className="sticky top-22 space-y-1">
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
                'flex h-12 w-full items-center gap-3 rounded-xl px-3 text-left text-sm font-medium transition-colors',
                isActive
                  ? 'bg-primary-500/10 text-primary-400'
                  : 'text-text-secondary hover:bg-bg-elevated/50 hover:text-text-primary',
              )}
            >
              <item.icon className={cn('size-4 shrink-0', isActive ? 'text-primary-400' : 'text-text-tertiary')} />
              <span className="truncate">{item.label}</span>
            </button>
          );
        })}
      </div>
    </nav>
  );
}
