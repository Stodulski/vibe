import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';

const t = ES_AR;

export type RevenuePeriod = 'week' | 'month';

interface PeriodToggleProps {
  period: RevenuePeriod;
  onChange: (period: RevenuePeriod) => void;
}

export function PeriodToggle({ period, onChange }: PeriodToggleProps) {
  return (
    <div
      className="flex rounded-lg border border-border-subtle bg-bg-base p-0.5"
      role="group"
      aria-label={t.dashboard.revenuePeriodLabel}
    >
      {(['week', 'month'] as const).map((p) => (
        <button
          key={p}
          onClick={() => {
            onChange(p);
          }}
          aria-pressed={period === p}
          className={cn(
            'rounded-lg px-3 py-1.5 text-xs font-medium transition-colors duration-150',
            period === p ? 'bg-bg-elevated text-text-primary' : 'text-text-tertiary hover:text-text-secondary',
          )}
        >
          {t.dashboard[p]}
        </button>
      ))}
    </div>
  );
}
