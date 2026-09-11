import { Building2, ChevronRight } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { ActiveStatusBadge } from '@/shared/components/common/ActiveStatusBadge';
import { TappableCard } from '@/shared/components/common/TappableCard';
import type { AdminUserRow } from '@/shared/types/api.types';
import { getRoleLabel } from './userTableUtils';

interface MobileUserCardProps {
  user: AdminUserRow;
  onClick: () => void;
}

export function MobileUserCard({ user, onClick }: MobileUserCardProps) {
  return (
    <TappableCard onTap={onClick}>
      <div className="flex size-10 shrink-0 items-center justify-center rounded-full bg-primary-500/10 text-sm font-semibold text-primary-500">
        {user.first_name[0]}
        {user.last_name[0]}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <p className="truncate text-sm font-medium text-text-primary">
            {user.first_name} {user.last_name}
          </p>
          {!user.is_active && <ActiveStatusBadge isActive={false} className="text-micro px-1.5 py-0" />}
        </div>
        <p className="truncate text-xs text-text-tertiary">{user.email}</p>
        <div className="mt-1 flex items-center gap-3 text-xs text-text-tertiary">
          <span className="flex items-center gap-1">
            <Building2 className="size-3" aria-hidden="true" />
            {user.complex_count}
          </span>
          <Badge variant="secondary" className="text-micro px-1.5 py-0">
            {getRoleLabel(user.role)}
          </Badge>
        </div>
      </div>
      <ChevronRight className="size-4 shrink-0 text-text-tertiary" aria-hidden="true" />
    </TappableCard>
  );
}
