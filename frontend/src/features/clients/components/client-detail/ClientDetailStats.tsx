import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;
interface ClientDetailStatsProps {
  totalBookings: number;
  noShows: number;
  attendance: number;
}

type Tone = 'neutral' | 'warning' | 'error' | 'success';

const TONE_CLASS: Record<Tone, string> = {
  neutral: 'text-text-primary',
  warning: 'text-warning-text',
  error: 'text-error-text',
  success: 'text-success-text',
};

/** Tone for the no-shows count: 3+ reads as a real problem, 1-2 as a warning worth noticing, 0 as neutral. */
function noShowsTone(noShows: number): Tone {
  if (noShows >= 3) return 'error';
  if (noShows > 0) return 'warning';
  return 'neutral';
}

/** Tone for the attendance rate: same three-band read as `noShowsTone`, good/watch/bad. */
function attendanceTone(attendance: number): Tone {
  if (attendance >= 80) return 'success';
  if (attendance >= 50) return 'warning';
  return 'error';
}

// `data-tone` exists for tests only, so they can assert the tone directly
// instead of pinning color classes. It does not fix WCAG 1.4.1 (color-only
// tone) — that is tracked separately (Engram #714).
function StatRow({ label, value, tone }: { label: string; value: string | number; tone?: Tone }) {
  return (
    <div>
      <p className="text-text-tertiary text-xs font-medium">{label}</p>
      <p data-tone={tone} className={cn('score-text mt-0.5 text-sm font-medium', tone && TONE_CLASS[tone])}>
        {value}
      </p>
    </div>
  );
}

export function ClientDetailStats({ totalBookings, noShows, attendance }: ClientDetailStatsProps) {
  return (
    <div className="space-y-3">
      <StatRow label={t.clients.bookings} value={totalBookings} />
      <StatRow label={t.clients.noShows} value={noShows} tone={noShowsTone(noShows)} />
      <StatRow label={t.clients.attendance} value={`${String(attendance)}%`} tone={attendanceTone(attendance)} />
    </div>
  );
}
