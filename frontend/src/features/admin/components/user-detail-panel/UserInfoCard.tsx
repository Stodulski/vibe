import { Building2, Mail, Phone, Calendar, Shield, CheckCircle, XCircle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Panel } from '@/shared/components/common/Panel';
import { Badge } from '@/shared/components/ui/badge';
import { Separator } from '@/shared/components/ui/separator';
import type { User } from '@/shared/types/api.types';
import { formatDateLong } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getRoleLabel } from '../users-table/userTableUtils';

const t = ES_AR;

interface UserDetailGridProps {
  user: User;
  complexesCount: number;
}

function UserDetailGrid({ user, complexesCount }: UserDetailGridProps) {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <div className="flex items-center gap-2 text-sm">
        <Mail className="text-text-tertiary size-4" />
        <span className="text-text-secondary">{user.email}</span>
      </div>
      <div className="flex items-center gap-2 text-sm">
        <Phone className="text-text-tertiary size-4" />
        <span className="text-text-secondary">{user.phone}</span>
      </div>
      <div className="flex items-center gap-2 text-sm">
        <Calendar className="text-text-tertiary size-4" />
        <span className="text-text-secondary">{formatDateLong(user.created_at)}</span>
      </div>
      <div className="flex items-center gap-2 text-sm">
        <Shield className="text-text-tertiary size-4" />
        <span className="text-text-secondary">{user.is_active ? t.admin.users.active : t.admin.users.inactive}</span>
        {user.is_active ? (
          <CheckCircle className="text-success-text size-3.5" />
        ) : (
          <XCircle className="text-error-text size-3.5" />
        )}
      </div>
      <div className="flex items-center gap-2 text-sm">
        <Badge variant={user.email_verified ? 'default' : 'secondary'} className="text-xs">
          {user.email_verified ? t.admin.users.verified : t.admin.users.notVerified}
        </Badge>
      </div>
      <div className="flex items-center gap-2 text-sm">
        <Building2 className="text-text-tertiary size-4" />
        <span className="text-text-secondary">
          {complexesCount} {complexesCount === 1 ? 'complejo' : 'complejos'}
        </span>
      </div>
    </div>
  );
}

interface UserInfoCardProps {
  user: User;
  complexesCount: number;
  isSelf: boolean;
  isTogglePending: boolean;
  onToggleClick: () => void;
}

export function UserInfoCard({ user, complexesCount, isSelf, isTogglePending, onToggleClick }: UserInfoCardProps) {
  return (
    <Panel size="md">
      <div className="flex items-start justify-between">
        <div>
          <h2 className="text-text-primary text-xl font-bold">
            {user.first_name} {user.last_name}
          </h2>
          <Badge variant="secondary" className="mt-1 text-xs">
            {getRoleLabel(user.role)}
          </Badge>
        </div>
        {!isSelf && (
          <Button
            variant={user.is_active ? 'destructive' : 'default'}
            size="sm"
            className="shrink-0 px-2.5 text-xs"
            onClick={onToggleClick}
            disabled={isTogglePending}
          >
            {user.is_active ? t.admin.users.deactivate : t.admin.users.activate}
          </Button>
        )}
      </div>

      <Separator className="my-4" />

      <UserDetailGrid user={user} complexesCount={complexesCount} />
    </Panel>
  );
}
