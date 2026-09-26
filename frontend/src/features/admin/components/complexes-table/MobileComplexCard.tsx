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
      <div className="bg-primary-500/10 text-primary-400 flex size-10 shrink-0 items-center justify-center rounded-full">
        <Trophy className="size-4" aria-hidden="true" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <p className="text-text-primary truncate text-sm font-medium">{complex.name}</p>
          {!complex.is_active && <ActiveStatusBadge isActive={false} className="text-micro px-1.5 py-0" />}
        </div>
        <p className="text-text-tertiary truncate text-xs">
          {complex.city} &middot; {complex.owner_name}
        </p>
        <div className="text-text-tertiary mt-1 flex items-center gap-3 text-xs">
          <span>
            {complex.courts_count} {t.admin.complexes.courts.toLowerCase()}
          </span>
          {complex.mp_connected && (
            <span className="text-success-text flex items-center gap-0.5">
              <CheckCircle className="size-3" aria-hidden="true" />
              MP
            </span>
          )}
        </div>
      </div>
      <ChevronRight className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
    </TappableCard>
  );
}
