import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { ChartTooltipState } from './useChartTooltip';

const t = ES_AR;

interface ChartTooltipProps {
  tooltip: ChartTooltipState;
  style: { left: number; top: number };
}

export function ChartTooltip({ tooltip, style }: ChartTooltipProps) {
  return (
    <div className="pointer-events-none absolute z-10 -translate-x-1/2" style={{ left: style.left, top: style.top }}>
      <div
        style={{
          backgroundColor: 'var(--color-bg-elevated)',
          border: '1px solid rgba(255,255,255,0.08)',
          borderRadius: '0.5rem',
          color: 'var(--color-text-primary)',
          boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
          padding: '8px 12px',
          fontSize: '13px',
          whiteSpace: 'nowrap',
        }}
      >
        <div style={{ color: 'var(--color-text-secondary)', fontSize: 12 }}>{tooltip.datum.date}</div>
        <div>
          {t.dashboard.revenue}: {formatPrice(tooltip.datum.amount)}
        </div>
      </div>
    </div>
  );
}
