import type { ComponentProps } from 'react';
import { cn } from '@/shared/lib/utils';

/**
 * Shared desktop table shell — the rounded/bordered container that wraps
 * every desktop-only data table (and its loading skeleton) across the app.
 * Purely a presentational wrapper; content is passed as children.
 */
export function DesktopTableShell({ className, children, ...props }: ComponentProps<'div'>) {
  return (
    <div className={cn('border-border-subtle bg-bg-subtle hidden rounded-2xl border md:block', className)} {...props}>
      {children}
    </div>
  );
}
