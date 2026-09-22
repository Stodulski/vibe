import { Link } from 'react-router-dom';
import { Building2 } from 'lucide-react';
import { ActiveStatusBadge } from '@/shared/components/common/ActiveStatusBadge';
import { Panel } from '@/shared/components/common/Panel';
import type { Complex } from '@/shared/types/api.types';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface OwnedComplexesListProps {
  complexes: Complex[];
}

export function OwnedComplexesList({ complexes }: OwnedComplexesListProps) {
  const complex = complexes[0] ?? null;

  return (
    <div>
      <h3 className="text-text-primary mb-3 flex items-center gap-2 text-base font-semibold">
        <Building2 className="size-4" />
        {t.admin.detail.ownedComplexes}
      </h3>
      {!complex ? (
        <Panel size="md">
          <p className="text-text-tertiary text-center text-sm">{t.admin.detail.noComplexes}</p>
        </Panel>
      ) : (
        <Link
          to={`/admin/complexes/${complex.id}`}
          className="focus-visible:ring-primary-500/50 block rounded-2xl focus-visible:ring-2 focus-visible:outline-none"
        >
          <Panel size="sm" className="hover:border-border-default p-3.5 transition-colors">
            <div className="flex items-start justify-between">
              <div>
                <p className="text-text-primary font-medium">{complex.name}</p>
                <p className="text-text-tertiary text-xs">
                  {complex.city}, {complex.province}
                </p>
                <p className="text-text-tertiary mt-1 text-xs">/{complex.slug}</p>
              </div>
              <ActiveStatusBadge isActive={complex.is_active} className="text-xs" />
            </div>
          </Panel>
        </Link>
      )}
    </div>
  );
}
