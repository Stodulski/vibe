import { ChevronRight } from 'lucide-react';
import { cn, formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { ClientInsights, TopClient } from '@/shared/types/api.types';

const t = ES_AR;

// Matches the backend's own LIMIT 10 (GetInsights, internal/data/clients.go)
// — always renders 10 rows so the card holds a stable height regardless of
// how many clients actually made the cut this period.
const TOP_ROWS = 10;

interface TopClientsListProps {
  top: ClientInsights['top'];
  onSelect: (client: TopClient) => void;
}

export function TopClientsList({ top, onSelect }: TopClientsListProps) {
  return (
    <div>
      <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-text-tertiary">{t.dashboard.topLoyal}</p>
      {top.length === 0 ? (
        <p className="py-2.5 text-xs text-text-tertiary">{t.dashboard.noTopClients}</p>
      ) : (
        <ul className="space-y-1">
          {Array.from({ length: TOP_ROWS }, (_, i) => top[i] ?? null).map((client, i) => (
            <li key={client?.id ?? `placeholder-${String(i)}`}>
              <button
                type="button"
                disabled={!client}
                onClick={() => {
                  if (client) onSelect(client);
                }}
                className="flex w-full items-center justify-between rounded-lg px-2 py-2.5 text-left transition-colors enabled:hover:bg-bg-base/40 disabled:cursor-default"
              >
                <div className="flex items-center gap-2 min-w-0">
                  <span
                    className={cn(
                      'score-text flex size-5 shrink-0 items-center justify-center rounded-full text-micro font-bold',
                      client ? 'bg-primary-500/10 text-primary-400' : 'bg-bg-base/60 text-text-disabled',
                    )}
                  >
                    {i + 1}
                  </span>
                  <span className={cn('truncate text-xs', client ? 'text-text-primary' : 'text-text-disabled')}>
                    {client?.name ?? '—'}
                  </span>
                </div>
                {client && (
                  <div className="flex shrink-0 items-center gap-3">
                    <span className="text-xs text-text-tertiary">{client.booking_count} res.</span>
                    <span className="score-text text-xs font-medium text-text-secondary">
                      {formatPrice(client.total_spent)}
                    </span>
                    <ChevronRight className="size-4 shrink-0 text-text-tertiary" aria-hidden="true" />
                  </div>
                )}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
