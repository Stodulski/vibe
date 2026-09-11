import { ES_AR } from '@/shared/i18n/es_AR';
import { Tag } from '@/shared/components/common/Tag';

const t = ES_AR;

/**
 * How a figure moved against the same figure from the period before.
 *
 * `versus` names that period out loud, and it is required rather than
 * defaulted: the accessible name used to say "vs ayer" no matter what was
 * being compared, which was true only because every caller happened to be a
 * day-over-day tile. A month-over-month badge that still announced "vs
 * yesterday" would read correctly and say something false.
 */
export function ComparisonBadge({
  current,
  previous,
  versus,
}: {
  current: number;
  previous: number;
  /** What the comparison is against, e.g. "ayer" or "el mes pasado". */
  versus: string;
}) {
  if (previous === 0) return null;

  const diff = current - previous;
  const pct = Math.round((diff / previous) * 100);

  if (pct === 0) return null;

  const isPositive = pct > 0;
  return (
    <Tag
      tone={isPositive ? 'success' : 'error'}
      className="gap-1 px-1.5 text-sm"
      role="status"
      aria-label={`${isPositive ? t.dashboard.increase : t.dashboard.decrease} de ${String(Math.abs(pct))}% vs ${versus}`}
    >
      <span aria-hidden="true">{isPositive ? '↑' : '↓'}</span>
      {Math.abs(pct)}%
    </Tag>
  );
}
