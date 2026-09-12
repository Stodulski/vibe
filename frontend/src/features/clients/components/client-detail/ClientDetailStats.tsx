import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;
interface ClientDetailStatsProps {
  totalBookings: number;
  noShows: number;
  attendance: number;
}

function StatRow({ label, value, valueClassName }: { label: string; value: string | number; valueClassName?: string }) {
  return (
    <div>
      <p className="text-text-tertiary text-xs font-medium">{label}</p>
      <p className={cn('score-text mt-0.5 text-sm font-medium', valueClassName)}>{value}</p>
    </div>
  );
}

export function ClientDetailStats({ totalBookings, noShows, attendance }: ClientDetailStatsProps) {
  return (
    <div className="space-y-3">
      <StatRow label={t.clients.bookings} value={totalBookings} />
      <StatRow
        label={t.clients.noShows}
        value={noShows}
        valueClassName={noShows >= 3 ? 'text-error-text' : noShows > 0 ? 'text-warning-text' : 'text-text-primary'}
      />
      <StatRow
        label={t.clients.attendance}
        value={`${String(attendance)}%`}
        valueClassName={
          attendance >= 80 ? 'text-success-text' : attendance >= 50 ? 'text-warning-text' : 'text-error-text'
        }
      />
    </div>
  );
}
