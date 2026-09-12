import { Link } from 'react-router-dom';
import { Panel } from '@/shared/components/common/Panel';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface OwnerInfoCardProps {
  ownerId: string;
  ownerName: string;
  ownerEmail: string;
}

export function OwnerInfoCard({ ownerId, ownerName, ownerEmail }: OwnerInfoCardProps) {
  return (
    <Panel size="md" className="sm:p-5">
      <h3 className="text-text-primary mb-2 text-sm font-semibold">{t.admin.detail.ownerInfo}</h3>
      <div className="flex items-center justify-between">
        <div>
          <p className="text-text-secondary text-sm">{ownerName}</p>
          <p className="text-text-tertiary text-xs">{ownerEmail}</p>
        </div>
        <Link to={`/admin/users/${ownerId}`} className="text-primary-400 text-xs hover:underline">
          {t.admin.detail.viewOwnerProfile}
        </Link>
      </div>
    </Panel>
  );
}
