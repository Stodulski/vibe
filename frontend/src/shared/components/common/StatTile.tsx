import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { cn } from '@/shared/lib/utils';
import { Panel } from './Panel';

// Every tone routes to the status/primary tokens (odd/tasks/app-dark-contrast.md
// T3) instead of the raw Tailwind default palette (`bg-purple-500/10`,
// `bg-orange-500/10`...), which bypassed `globals.css` entirely. `blue`,
// `purple`, `orange` and `emerald` are decorative aliases kept for the
// existing call sites (`ComplexStatsGrid`) that use them to tell a few KPI
// tiles apart in one grid — they are not semantic status claims about the
// metric, just distinct on-token colors. `cyan`/`pink`/`yellow`/`green` had
// zero call sites (verified with `rg`) and were dropped rather than given a
// token mapping nobody uses.
export type StatTileTone = 'primary' | 'info' | 'warning' | 'success' | 'blue' | 'purple' | 'orange' | 'emerald';

const STAT_TILE_TONES: Record<StatTileTone, string> = {
  primary: 'bg-primary-500/10 text-primary-400',
  info: 'bg-info-bg text-info-icon',
  warning: 'bg-warning-bg text-warning-icon',
  success: 'bg-success-bg text-success-icon',
  blue: 'bg-info-bg text-info-icon',
  purple: 'bg-primary-500/10 text-primary-400',
  orange: 'bg-warning-bg text-warning-icon',
  emerald: 'bg-success-bg text-success-icon',
};

interface StatTileProps {
  label: string;
  value: ReactNode;
  icon?: LucideIcon;
  iconTone?: StatTileTone;
  subtitle?: string;
  /** Dashboard-only delta badge (e.g. `ComparisonBadge`), rendered next to the label. */
  comparison?: ReactNode;
  /** Dashboard-only 0-100 progress bar rendered below the value. */
  progressBar?: number;
  valueClassName?: string;
  className?: string;
}

/**
 * The 0-100 progress bar rendered below a `StatTile`'s value. Split out so
 * the clamped percentage (`pct`) is computed once and used for both the
 * announced `aria-valuenow` and the rendered width, which never disagree.
 */
function StatTileProgressBar({ label, progressBar }: { label: string; progressBar: number }) {
  const pct = Math.min(Math.max(progressBar, 0), 100);
  return (
    <div
      className="bg-bg-base mt-2 h-1 w-full overflow-hidden rounded-full sm:mt-3"
      role="progressbar"
      aria-valuenow={pct}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label={`${label}: ${String(pct)}%`}
    >
      <div
        className="bg-primary-500 h-1 rounded-full transition-[width] duration-700 ease-out"
        style={{ width: `${String(pct)}%` }}
      />
    </div>
  );
}

/**
 * Unified "metric tile" (icon + label + value) — consolidates the 4
 * hand-rolled variants that used to live in dashboard/admin/clients
 * (different radii, layouts, and icon-badge treatments). Built on `Panel`
 * for the shared card recipe; `icon`/`comparison`/`progressBar` are all
 * optional so simpler call sites (e.g. client detail) just omit them.
 */
export function StatTile({
  label,
  value,
  icon: Icon,
  iconTone = 'primary',
  subtitle,
  comparison,
  progressBar,
  valueClassName,
  className,
}: StatTileProps) {
  return (
    // A stat tile always sits beside its neighbors in a dense grid
    // (`gap-2`/`gap-3`) rather than stacked with page-level spacing between
    // them — without its own box it would visually run into the tile next to
    // it on a phone (odd/tasks/app-dark-contrast.md T2 Panel audit).
    <Panel as="article" mobile="card" className={cn('hover-lift', className)}>
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <span className="text-text-tertiary text-sm font-medium">{label}</span>
          {comparison}
        </div>
        {Icon && (
          <div className={cn('flex size-8 shrink-0 items-center justify-center rounded-lg', STAT_TILE_TONES[iconTone])}>
            <Icon className="size-4" aria-hidden="true" />
          </div>
        )}
      </div>
      <div className="mt-2 flex items-baseline gap-2 sm:mt-3">
        <p
          className={cn(
            'animate-count-in text-text-primary text-lg font-bold tracking-tight whitespace-nowrap sm:text-xl',
            valueClassName,
          )}
        >
          {value}
        </p>
      </div>
      {subtitle && <p className="text-text-tertiary mt-1 text-sm whitespace-nowrap">{subtitle}</p>}
      {progressBar != null && <StatTileProgressBar label={label} progressBar={progressBar} />}
    </Panel>
  );
}
