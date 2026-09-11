import { ES_AR } from '@/shared/i18n/es_AR';
import type { ClientInsights } from '@/shared/types/api.types';

const t = ES_AR;

export function ClientMetricsRow({ data }: { data: ClientInsights }) {
  return (
    <div className="mb-4 grid grid-cols-3 gap-2">
      <div className="rounded-lg bg-bg-base/40 px-2.5 py-2 text-center">
        <p className="score-text text-lg font-bold text-text-primary">{data.no_show_rate}%</p>
        <p className="text-xs text-text-tertiary">{t.clients.noShows}</p>
      </div>
      <div className="rounded-lg bg-bg-base/40 px-2.5 py-2 text-center">
        <p className="score-text text-lg font-bold text-text-primary">{data.new_clients_30d}</p>
        <p className="text-xs text-text-tertiary">{t.dashboard.newClients}</p>
      </div>
      <div className="rounded-lg bg-bg-base/40 px-2.5 py-2 text-center">
        <p className="score-text text-lg font-bold text-text-primary">{data.recurring_30d}</p>
        <p className="text-xs text-text-tertiary">{t.dashboard.recurringClients}</p>
      </div>
    </div>
  );
}
