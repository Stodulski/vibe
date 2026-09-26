import type { ComponentProps } from 'react';
import { cn } from '@/shared/lib/utils';

type PanelSize = 'sm' | 'md' | 'lg';
/**
 * `flat` (the default): no card below `sm` — a phone screen is mostly
 * cardless (odd/tasks/app-dark-contrast.md T2). `card` opts a specific
 * content out of that, for the handful of usages that are a tappable tile or
 * otherwise need a box at every width (a stat tile in a grid, a tappable row
 * card) rather than a plain content section.
 */
type PanelMobile = 'flat' | 'card';

const PANEL_SIZES: Record<PanelSize, Record<PanelMobile, string>> = {
  sm: { flat: 'py-3 sm:p-4', card: 'p-3 sm:p-4' },
  md: { flat: 'py-4 sm:p-6', card: 'p-4 sm:p-6' },
  lg: { flat: 'py-5 sm:p-8', card: 'p-5 sm:p-8' },
};

// No horizontal padding below `sm` (it costs width a 320px phone doesn't
// have to spend) and no border/fill/radius — those come back from `sm:` up.
// `card` keeps the box at every width, unchanged from what `Panel` always
// rendered.
const PANEL_SURFACE: Record<PanelMobile, string> = {
  flat: 'rounded-none border-0 bg-transparent sm:rounded-2xl sm:border sm:border-border-subtle sm:bg-bg-subtle',
  card: 'rounded-2xl border border-border-subtle bg-bg-subtle',
};

interface PanelProps extends ComponentProps<'div'> {
  size?: PanelSize;
  /** @default 'flat' */
  mobile?: PanelMobile;
  // Most call sites are plain containers, but a few are landmark-ish
  // sections/cards that want their semantic tag preserved.
  as?: 'div' | 'section' | 'article';
}

/**
 * Shared content-section recipe. Below `sm` it is flat and full-width — a
 * plain container separated from its neighbors by the page's own spacing and
 * its own heading, not a box (odd/tasks/app-dark-contrast.md T2). From `sm:`
 * up (and always, when `mobile="card"`) it is the `rounded-2xl border
 * border-border-subtle bg-bg-subtle` card recipe hand-rolled across 25+
 * dashboard/owner/public-booking/admin sites. `size` captures the real
 * density variants observed across those sites (sm/md/lg), defaulting to
 * `md` (the most common padding).
 */
export function Panel({ size = 'md', mobile = 'flat', as: Tag = 'div', className, ...props }: PanelProps) {
  return <Tag className={cn(PANEL_SURFACE[mobile], PANEL_SIZES[size][mobile], className)} {...props} />;
}
