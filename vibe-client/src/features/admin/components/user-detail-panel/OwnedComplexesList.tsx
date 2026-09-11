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
  return (
    <div>
      <h3 className="mb-3 text-base font-semibold text-text-primary flex items-center gap-2">
        <Building2 className="size-4" />
        {t.admin.detail.ownedComplexes} ({complexes.length})
      </h3>
      {complexes.length === 0 ? (
        <Panel size="md">
          <p className="text-center text-sm text-text-tertiary">{t.admin.detail.noComplexes}</p>
        </Panel>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {complexes.map((complex) => (
            <Link
              key={complex.id}
              to={`/admin/complexes/${complex.id}`}
              className="block rounded-2xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50"
            >
              <Panel size="sm" className="p-3.5 transition-colors hover:border-border-default">
                <div className="flex items-start justify-between">
                  <div>
                    <p className="font-medium text-text-primary">{complex.name}</p>
                    <p className="text-xs text-text-tertiary">
                      {complex.city}, {complex.province}
                    </p>
                    <p className="mt-1 text-xs text-text-tertiary">/{complex.slug}</p>
                  </div>
                  <ActiveStatusBadge isActive={complex.is_active} className="text-xs" />
                </div>
              </Panel>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
