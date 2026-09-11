import type { ComponentProps, ReactNode } from 'react';
import { cn } from '@/shared/lib/utils';

const DEFAULT_BUTTON_CLASSES =
  'flex w-full items-center gap-3 rounded-xl border border-border-subtle bg-bg-subtle p-3.5 text-left transition-colors hover:border-border-default active:scale-[0.99]';

const DEFAULT_ACTION_CLASSES = 'absolute right-3.5 top-1/2 -translate-y-1/2';

interface TappableCardProps extends Omit<ComponentProps<'div'>, 'onClick'> {
  /** Called when the card body is activated (click or native button keyboard activation). */
  onTap: () => void;
  /**
   * Trailing action (e.g. a dropdown-menu trigger, quick-action buttons). Rendered
   * as a sibling of the native button — NEVER as a descendant — since nesting an
   * interactive element inside a <button> is invalid HTML and breaks a11y.
   */
  action?: ReactNode;
  /**
   * Override the button's own classes. Defaults to the standard compact row
   * (icon/text + p-3.5 padding). Only needed when a site's internal content
   * already owns its own padding/dividers (e.g. a card split into columns).
   */
  buttonClassName?: string;
  /** Override the action wrapper's positioning classes (defaults to an overlay pinned to the button's right edge, vertically centered). */
  actionClassName?: string;
  /**
   * The card's accessible name. Worth setting whenever the body holds more
   * than a label — a card carrying a name and two figures reads out as a
   * run-on of every number in it otherwise, and the thing it opens is the
   * one named at the top.
   */
  buttonLabel?: string;
  children: ReactNode;
}

/**
 * Shared "mobile tappable card" — a native <button> for the tappable content
 * plus an optional trailing action rendered as a sibling (not nested inside
 * the button), so the card gets real native focus/keyboard semantics without
 * producing invalid interactive-in-interactive HTML when an action menu is
 * also present.
 */
export function TappableCard({
  onTap,
  action,
  buttonLabel,
  buttonClassName,
  actionClassName,
  children,
  className,
  ...props
}: TappableCardProps) {
  return (
    <div className={cn('relative', className)} {...props}>
      <button
        type="button"
        onClick={onTap}
        aria-label={buttonLabel}
        className={buttonClassName ?? DEFAULT_BUTTON_CLASSES}
      >
        {children}
      </button>
      {action && <div className={actionClassName ?? DEFAULT_ACTION_CLASSES}>{action}</div>}
    </div>
  );
}
