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

interface UserMenuTriggerProps {
  user: ApiUser | null;
  collapsed: boolean;
  isMobile: boolean;
  fullName: string;
}

function UserMenuTrigger({ user, collapsed, isMobile, fullName }: UserMenuTriggerProps) {
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
          /* A person, not an overflow dot column. Collapsed there is no email
             beside it, so the glyph has to say WHOSE menu this is — and a lone
             "⋮" on a rail of subject icons says only "more of something". The
             expanded row keeps the dots, where the email is the subject and
             the dots are genuinely the overflow beside it. */
          <User className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
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

interface UserMenuItemsProps {
  collapsed: boolean;
  isMobile: boolean;
  onProfileClick: () => void;
  onLogoutClick: () => void;
}

function UserMenuItems({ collapsed, isMobile, onProfileClick, onLogoutClick }: UserMenuItemsProps) {
  return (
    <DropdownMenuContent
      align={collapsed && !isMobile ? 'start' : 'end'}
      side={collapsed && !isMobile ? 'right' : 'top'}
      className="w-56"
    >
      <DropdownMenuItem onClick={onProfileClick}>
        <User aria-hidden="true" />
        {t.navigation.profile}
      </DropdownMenuItem>
      <DropdownMenuSeparator />
      <DropdownMenuItem variant="destructive" onClick={onLogoutClick}>
        <LogOut aria-hidden="true" />
        {t.auth.logout}
      </DropdownMenuItem>
    </DropdownMenuContent>
  );
}

interface SidebarUserMenuProps {
  user: ApiUser | null;
  collapsed: boolean;
  isMobile: boolean;
  onNavigate?: (() => void) | undefined;
}

export function SidebarUserMenu({ user, collapsed, isMobile, onNavigate }: SidebarUserMenuProps) {
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
        <UserMenuTrigger user={user} collapsed={collapsed} isMobile={isMobile} fullName={fullName} />
        <UserMenuItems
          collapsed={collapsed}
          isMobile={isMobile}
          onProfileClick={() => {
            onNavigate?.();
            void navigate('/profile');
          }}
          onLogoutClick={() => {
            logout.mutate();
          }}
        />
      </DropdownMenu>
    </div>
  );
}
