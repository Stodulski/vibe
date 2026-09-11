import type { ComponentProps } from 'react';
import { cn } from '@/shared/lib/utils';

export type TagTone = 'primary' | 'success' | 'error' | 'info' | 'neutral';

const TAG_TONES: Record<TagTone, string> = {
  primary: 'bg-primary-500/10 text-primary-400',
  success: 'bg-success-bg text-success-text',
  error: 'bg-error-bg text-error-text',
  info: 'bg-info-bg text-info-text',
  neutral: 'bg-bg-elevated text-text-secondary',
};

interface TagProps extends ComponentProps<'span'> {
  tone?: TagTone;
}

/**
 * Small ad-hoc label/counter tag — a dumb presentational wrapper for the
 * one-off pill spans scattered across the app (counts, "today", "next day",
 * "blocked", % deltas, etc). Not a status-mapping abstraction like
 * StatusBadges/Badge — those already own their own semantics; this is only
 * for spans that had zero shared component before.
 */
export function Tag({ tone = 'neutral', className, children, ...props }: TagProps) {
  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center whitespace-nowrap rounded-md px-2 py-0.5 text-sm font-medium',
        TAG_TONES[tone],
        className,
      )}
      {...props}
    >
      {children}
    </span>
  );
}
