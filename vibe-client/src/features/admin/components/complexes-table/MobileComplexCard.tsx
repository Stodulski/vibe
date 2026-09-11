import { CheckCircle, ChevronRight, Trophy } from 'lucide-react';
import { ActiveStatusBadge } from '@/shared/components/common/ActiveStatusBadge';
import { TappableCard } from '@/shared/components/common/TappableCard';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { AdminComplexRow } from '@/shared/types/api.types';

const t = ES_AR;

interface MobileComplexCardProps {
  complex: AdminComplexRow;
  onClick: () => void;
}

export function MobileComplexCard({ complex, onClick }: MobileComplexCardProps) {
  return (
    <TappableCard onTap={onClick}>
      <div className="flex size-10 shrink-0 items-center justify-center rounded-full bg-purple-500/10 text-purple-400">
        <Trophy className="size-4" aria-hidden="true" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <p className="truncate text-sm font-medium text-text-primary">{complex.name}</p>
          {!complex.is_active && <ActiveStatusBadge isActive={false} className="text-micro px-1.5 py-0" />}
        </div>
        <p className="truncate text-xs text-text-tertiary">
          {complex.city} &middot; {complex.owner_name}
        </p>
        <div className="mt-1 flex items-center gap-3 text-xs text-text-tertiary">
          <span>
            {complex.courts_count} {t.admin.complexes.courts.toLowerCase()}
          </span>
          {complex.mp_connected && (
            <span className="flex items-center gap-0.5 text-green-400">
              <CheckCircle className="size-3" aria-hidden="true" />
              MP
            </span>
          )}
        </div>
      </div>
      <ChevronRight className="size-4 shrink-0 text-text-tertiary" aria-hidden="true" />
    </TappableCard>
  );
}
