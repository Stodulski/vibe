import { Link } from 'react-router-dom';
import { useCourts } from '@/shared/hooks/useCourts';
import { uncoveredOpeningHours } from '@/shared/lib/priceCoverage';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Schedule } from '@/shared/types/api.types';

const t = ES_AR;

interface PriceCoverageWarningProps {
  complexId: string;
  /** The week as the form holds it, so the warning shows before saving too. */
  schedule: Pick<Schedule, 'day' | 'open_time' | 'close_time' | 'is_closed'>[];
}

/**
 * Names the opening hours no price covers, per court. Changing the hours never
 * touches the price bands, so a day opened later than its prices reach is open
 * on paper and never offered for booking — until this said so.
 */
export function PriceCoverageWarning({ complexId, schedule }: PriceCoverageWarningProps) {
  const { data: courts } = useCourts(complexId);
  const gaps = uncoveredOpeningHours(courts ?? [], schedule);
  if (gaps.length === 0) return null;

  return (
    <div
      role="alert"
      className="border-warning-border bg-warning-bg text-warning-text space-y-2 rounded-xl border px-3.5 py-2.5 text-sm"
    >
      <p className="font-medium">{t.complex.priceCoverageTitle}</p>
      <p>{t.complex.priceCoverageDescription}</p>
      <ul className="space-y-0.5">
        {gaps.map((gap) => (
          <li key={`${gap.courtId}-${gap.day}-${gap.from}`} className="break-words">
            {gap.courtName}: {t.complex.days[gap.day]} {gap.from} {t.complex.timeRangeJoiner} {gap.to}
          </li>
        ))}
      </ul>
      <Link to="/courts" className="inline-block font-medium underline underline-offset-2">
        {t.complex.priceCoverageAction}
      </Link>
    </div>
  );
}
