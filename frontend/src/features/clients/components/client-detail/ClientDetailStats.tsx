import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;
interface ClientDetailStatsProps {
  totalBookings: number;
  noShows: number;
  attendance: number;
}

/** Tone for the no-shows count: 3+ reads as a real problem, 1-2 as a warning worth noticing, 0 as neutral. */
function noShowsTone(noShows: number): string {
  if (noShows >= 3) return 'text-error-text';
  if (noShows > 0) return 'text-warning-text';
  return 'text-text-primary';
}

/** Tone for the attendance rate: same three-band read as `noShowsTone`, good/watch/bad. */
function attendanceTone(attendance: number): string {
  if (attendance >= 80) return 'text-success-text';
  if (attendance >= 50) return 'text-warning-text';
  return 'text-error-text';
}

function StatRow({ label, value, valueClassName }: { label: string; value: string | number; valueClassName?: string }) {
  return (
    <div>
      <p className="text-xs font-medium text-text-tertiary">{label}</p>
      <p className={cn('score-text mt-0.5 text-sm font-medium', valueClassName)}>{value}</p>
    </div>
  );
}

export function ClientDetailStats({ totalBookings, noShows, attendance }: ClientDetailStatsProps) {
  return (
    <div className="space-y-3">
      <StatRow label={t.clients.bookings} value={totalBookings} />
      <StatRow label={t.clients.noShows} value={noShows} valueClassName={noShowsTone(noShows)} />
      <StatRow
        label={t.clients.attendance}
        value={`${String(attendance)}%`}
        valueClassName={attendanceTone(attendance)}
      />
    </div>
  );
}
