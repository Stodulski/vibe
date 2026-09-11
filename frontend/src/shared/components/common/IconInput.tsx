import type { ComponentProps, ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { cn } from '@/shared/lib/utils';

interface IconInputProps extends ComponentProps<typeof Input> {
  /** Leading icon, rendered inside a focus-aware wrapper around the input. */
  icon: LucideIcon;
  /** Overrides the wrapper's className (defaults to `relative flex-1`). */
  wrapperClassName?: string;
  /**
   * Overrides the default `<Input className="pl-9" {...props} />` with a
   * custom input element (e.g. one that needs extra classes/attrs beyond
   * what this component forwards). Used internally by `FormField`.
   */
  children?: ReactNode;
}

/**
 * Icon-prefixed input — extracted from the icon-wrapper treatment inside
 * `FormField` (which also owns label+error chrome this component doesn't
 * need). Used for label-less search/filter bars: same focus-within ring and
 * icon-color-on-focus polish, no label/error slots.
 */
export function IconInput({
  icon: Icon,
  wrapperClassName = 'relative flex-1',
  className,
  children,
  ...props
}: IconInputProps) {
  return (
    <div
      className={cn(
        'group rounded-lg transition-colors focus-within:ring-1 focus-within:ring-primary-500/30',
        wrapperClassName,
      )}
    >
      <Icon
        className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-text-tertiary transition-colors group-focus-within:text-primary-500/70"
        aria-hidden="true"
      />
      {children ?? <Input className={cn('pl-9', className)} {...props} />}
    </div>
  );
}
