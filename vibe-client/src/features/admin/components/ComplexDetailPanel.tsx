import { Link } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import type { AdminComplexDetailResponse } from '@/shared/types/api.types';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ComplexInfoCard } from './complex-detail-panel/ComplexInfoCard';
import { ComplexStatsGrid } from './complex-detail-panel/ComplexStatsGrid';
import { OwnerInfoCard } from './complex-detail-panel/OwnerInfoCard';

const t = ES_AR;

interface ComplexDetailPanelProps {
  data: AdminComplexDetailResponse;
}

export function ComplexDetailPanel({ data }: ComplexDetailPanelProps) {
  const { complex } = data;

  return (
    <div className="space-y-6 animate-fade-in">
      <Link
        to="/admin/complexes"
        className="flex items-center gap-1.5 text-sm text-text-tertiary hover:text-text-secondary transition-colors"
      >
        <ArrowLeft className="size-4" />
        {t.admin.detail.backToComplexes}
      </Link>

      <ComplexInfoCard complex={complex} />
      <ComplexStatsGrid data={data} />
      <OwnerInfoCard ownerId={complex.owner_id} ownerName={data.owner_name} ownerEmail={data.owner_email} />
    </div>
  );
}
