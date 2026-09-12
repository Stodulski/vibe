import { Building2, Mail, ExternalLink } from 'lucide-react';
import { ActiveStatusBadge } from '@/shared/components/common/ActiveStatusBadge';
import { Panel } from '@/shared/components/common/Panel';
import { Separator } from '@/shared/components/ui/separator';
import type { Complex } from '@/shared/types/api.types';
import { formatDateLong } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ComplexInfoCardProps {
  complex: Complex;
}

export function ComplexInfoCard({ complex }: ComplexInfoCardProps) {
  return (
    <Panel size="md">
      <div className="flex items-start justify-between">
        <div>
          <div className="flex items-center gap-2">
            <Building2 className="text-primary-500 size-5" />
            <h2 className="text-text-primary text-xl font-bold">{complex.name}</h2>
          </div>
          <p className="text-text-tertiary mt-1 text-sm">
            {complex.city}, {complex.province}
          </p>
          <p className="text-text-tertiary text-xs">/{complex.slug}</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <ActiveStatusBadge isActive={complex.is_active} className="text-xs" />
          <a
            href={`/${complex.slug}`}
            target="_blank"
            rel="noopener noreferrer"
            className="text-primary-400 flex shrink-0 items-center gap-1 text-xs hover:underline"
          >
            <span className="hidden sm:inline">{t.admin.detail.viewPublicPage}</span>
            <ExternalLink className="size-3.5" />
          </a>
        </div>
      </div>

      <Separator className="my-4" />

      <div className="grid gap-3 sm:grid-cols-2">
        <div className="flex items-center gap-2 text-sm">
          <Mail className="text-text-tertiary size-4" />
          <span className="text-text-secondary">{complex.email ?? complex.phone}</span>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <span className="text-text-tertiary">{t.admin.complexes.created}:</span>
          <span className="text-text-secondary">{formatDateLong(complex.created_at)}</span>
        </div>
        {complex.address && (
          <div className="text-text-secondary text-sm break-words sm:col-span-2">{complex.address}</div>
        )}
      </div>
    </Panel>
  );
}
