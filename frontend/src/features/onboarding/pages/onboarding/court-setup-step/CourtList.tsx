import { CheckCircle2, Trash2 } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Court } from '@/shared/types/api.types';

const t = ES_AR;

type CourtLike = Pick<Court, 'id' | 'name' | 'sport' | 'court_type'>;

export function CourtList({
  courts,
  onRemove,
  removePending,
}: {
  courts: CourtLike[];
  onRemove: (courtId: string) => void;
  removePending: boolean;
}) {
  return (
    <div className="mb-5 space-y-2 sm:mb-6">
      {courts.map((court, i) => (
        <div
          key={court.id}
          className="border-primary-500/20 bg-primary-500/5 flex items-center justify-between gap-3 rounded-xl border px-4 py-3"
          style={{ animationDelay: `${String(i * 50)}ms` }}
        >
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div className="bg-primary-500/15 flex size-8 shrink-0 items-center justify-center rounded-lg">
              <CheckCircle2 className="text-primary-400 size-4" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-text-primary truncate text-sm font-medium">{court.name}</p>
              <p className="text-text-tertiary text-xs">
                {t.courts.sportTypes[court.sport]} · {t.courts.courtTypes[court.court_type]}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={() => {
              onRemove(court.id);
            }}
            disabled={removePending}
            aria-label={`${t.common.delete}: ${court.name}`}
            className="text-text-tertiary hover:bg-error-bg hover:text-error-text flex size-8 shrink-0 items-center justify-center rounded-lg transition-colors disabled:opacity-50"
          >
            <Trash2 className="size-3.5" />
          </button>
        </div>
      ))}
    </div>
  );
}
