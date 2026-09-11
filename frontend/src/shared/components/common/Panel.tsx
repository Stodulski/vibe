import type { ComponentProps } from 'react';
import { cn } from '@/shared/lib/utils';

export type PanelSize = 'sm' | 'md' | 'lg';

const PANEL_SIZES: Record<PanelSize, string> = {
  sm: 'p-3 sm:p-4',
  md: 'p-4 sm:p-6',
  lg: 'p-5 sm:p-8',
};

interface PanelProps extends ComponentProps<'div'> {
  size?: PanelSize;
  // Most call sites are plain containers, but a few are landmark-ish
  // sections/cards that want their semantic tag preserved.
  as?: 'div' | 'section' | 'article';
}

/**
 * Shared card container recipe — the `rounded-2xl border border-border-subtle
 * bg-bg-subtle` box hand-rolled across 25+ dashboard/owner/public-booking/admin
 * sites. `size` captures the real density variants observed across those
 * sites (sm/md/lg), defaulting to `md` (the most common padding).
 */
export function Panel({ size = 'md', as: Tag = 'div', className, ...props }: PanelProps) {
  return (
    <Tag
      className={cn('rounded-2xl border border-border-subtle bg-bg-subtle', PANEL_SIZES[size], className)}
      {...props}
    />
  );
}
