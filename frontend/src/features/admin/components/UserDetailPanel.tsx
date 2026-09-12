import { Link } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { useState } from 'react';
import { useStore } from '@/shared/stores';
import { useToggleUserActive } from '../hooks/useToggleUserActive';
import type { User, Complex } from '@/shared/types/api.types';
import { ES_AR } from '@/shared/i18n/es_AR';
import { UserInfoCard } from './user-detail-panel/UserInfoCard';
import { OwnedComplexesList } from './user-detail-panel/OwnedComplexesList';

const t = ES_AR;

interface UserDetailPanelProps {
  user: User;
  complexes: Complex[];
}

export function UserDetailPanel({ user, complexes }: UserDetailPanelProps) {
  const { user: currentUser } = useStore();
  const toggleActive = useToggleUserActive();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const isSelf = currentUser?.id === user.id;

  return (
    <div className="animate-fade-in space-y-6">
      <Link
        to="/admin/users"
        className="text-text-tertiary hover:text-text-secondary flex items-center gap-1.5 text-sm transition-colors"
      >
        <ArrowLeft className="size-4" />
        {t.admin.detail.backToUsers}
      </Link>

      <UserInfoCard
        user={user}
        complexesCount={complexes.length}
        isSelf={isSelf}
        isTogglePending={toggleActive.isPending}
        onToggleClick={() => {
          setConfirmOpen(true);
        }}
      />

      <OwnedComplexesList complexes={complexes} />

      <ConfirmDialog
        open={confirmOpen}
        onClose={() => {
          setConfirmOpen(false);
        }}
        onConfirm={() => {
          setConfirmOpen(false);
          toggleActive.mutate({ userId: user.id, isActive: !user.is_active });
        }}
        title={user.is_active ? t.admin.users.deactivate : t.admin.users.activate}
        description={user.is_active ? t.admin.users.deactivateConfirm : t.admin.users.activateConfirm}
        confirmLabel={t.common.confirm}
        variant={user.is_active ? 'destructive' : 'default'}
      />
    </div>
  );
}
