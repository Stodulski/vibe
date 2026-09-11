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
            <Building2 className="size-5 text-primary-500" />
            <h2 className="text-xl font-bold text-text-primary">{complex.name}</h2>
          </div>
          <p className="mt-1 text-sm text-text-tertiary">
            {complex.city}, {complex.province}
          </p>
          <p className="text-xs text-text-tertiary">/{complex.slug}</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <ActiveStatusBadge isActive={complex.is_active} className="text-xs" />
          <a
            href={`/${complex.slug}`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex shrink-0 items-center gap-1 text-xs text-primary-400 hover:underline"
          >
            <span className="hidden sm:inline">{t.admin.detail.viewPublicPage}</span>
            <ExternalLink className="size-3.5" />
          </a>
        </div>
      </div>

      <Separator className="my-4" />

      <div className="grid gap-3 sm:grid-cols-2">
        <div className="flex items-center gap-2 text-sm">
          <Mail className="size-4 text-text-tertiary" />
          <span className="text-text-secondary">{complex.email ?? complex.phone}</span>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <span className="text-text-tertiary">{t.admin.complexes.created}:</span>
          <span className="text-text-secondary">{formatDateLong(complex.created_at)}</span>
        </div>
        {complex.address && (
          <div className="break-words sm:col-span-2 text-sm text-text-secondary">{complex.address}</div>
        )}
      </div>
    </Panel>
  );
}
