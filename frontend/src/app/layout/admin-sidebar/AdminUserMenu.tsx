import { useNavigate } from 'react-router-dom';
import { EllipsisVertical, LogOut, User } from 'lucide-react';
import { useLogout } from '@/features/auth/hooks/useLogout';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import type { User as ApiUser } from '@/shared/types/api.types';

const t = ES_AR;

interface AdminUserMenuTriggerProps {
  user: ApiUser | null;
  collapsed: boolean;
  isMobile: boolean;
  fullName: string;
}

function AdminUserMenuTrigger({ user, collapsed, isMobile, fullName }: AdminUserMenuTriggerProps) {
  return (
    <DropdownMenuTrigger asChild>
      <button
        className={cn(
          'hover:bg-bg-elevated/50 flex w-full items-center rounded-lg transition-colors',
          collapsed && !isMobile ? 'justify-center p-2' : 'gap-2 px-2 py-2 text-left',
        )}
        aria-label={`${t.layout.userMenuLabel}: ${fullName}`}
      >
        {collapsed && !isMobile ? (
          <EllipsisVertical className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
        ) : (
          <>
            <span className="text-nav text-text-secondary min-w-0 flex-1 truncate">{user?.email}</span>
            <EllipsisVertical className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
          </>
        )}
      </button>
    </DropdownMenuTrigger>
  );
}

interface AdminUserMenuProps {
  user: ApiUser | null;
  collapsed: boolean;
  isMobile: boolean;
  onNavigate?: (() => void) | undefined;
}

export function AdminUserMenu({ user, collapsed, isMobile, onNavigate }: AdminUserMenuProps) {
  const navigate = useNavigate();
  const logout = useLogout();

  const fullName = user ? `${user.first_name} ${user.last_name}` : '';

  return (
    <div
      className={cn(
        'border-border-subtle shrink-0 border-t',
        collapsed && !isMobile ? 'p-2' : 'p-3',
        isMobile && 'pb-[calc(0.75rem+env(safe-area-inset-bottom,0px))]',
      )}
    >
      <DropdownMenu>
        <AdminUserMenuTrigger user={user} collapsed={collapsed} isMobile={isMobile} fullName={fullName} />
        <DropdownMenuContent
          align={collapsed && !isMobile ? 'start' : 'end'}
          side={collapsed && !isMobile ? 'right' : 'top'}
          className="w-56"
        >
          <DropdownMenuItem
            onClick={() => {
              onNavigate?.();
              void navigate('/profile');
            }}
          >
            <User aria-hidden="true" />
            {t.navigation.profile}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            variant="destructive"
            onClick={() => {
              logout.mutate();
            }}
          >
            <LogOut aria-hidden="true" />
            {t.auth.logout}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
