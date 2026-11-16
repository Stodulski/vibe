import { useNavigate } from 'react-router-dom';
import { BookOpen, EllipsisVertical, LogOut, Settings, User } from 'lucide-react';
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

/** The step-by-step tutorials on the landing (owner request: "Manual de uso" in the menu). */
const USER_MANUAL_URL = 'https://vibe.com.ar/tutoriales';

interface UserMenuTriggerProps {
  isMobile: boolean;
  fullName: string;
}

/** The account entry reads "Mi cuenta" beside a profile icon (owner request); the mobile sheet keeps the overflow dots. */
function UserMenuTrigger({ isMobile, fullName }: UserMenuTriggerProps) {
  return (
    <DropdownMenuTrigger asChild>
      <button
        className="hover:bg-bg-elevated/50 flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left transition-colors"
        // Starts with the visible text so the accessible name contains it (WCAG 2.5.3); the person follows.
        aria-label={`${t.navigation.account}: ${fullName}`}
      >
        <User className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
        <span className="text-nav text-text-secondary min-w-0 flex-1 truncate">{t.navigation.account}</span>
        {isMobile && <EllipsisVertical className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />}
      </button>
    </DropdownMenuTrigger>
  );
}

interface UserMenuItemsProps {
  onProfileClick: () => void;
  onLogoutClick: () => void;
}

function UserMenuItems({ onProfileClick, onLogoutClick }: UserMenuItemsProps) {
  return (
    <DropdownMenuContent align="end" side="top" className="w-56">
      <DropdownMenuItem onClick={onProfileClick}>
        <Settings aria-hidden="true" />
        {t.navigation.accountSettings}
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
  isMobile: boolean;
  onNavigate?: (() => void) | undefined;
}

export function SidebarUserMenu({ user, isMobile, onNavigate }: SidebarUserMenuProps) {
  const navigate = useNavigate();
  const logout = useLogout();

  const fullName = user ? `${user.first_name} ${user.last_name}` : '';

  return (
    <div
      className={cn(
        'border-border-subtle shrink-0 border-t',
        'p-3',
        isMobile && 'pb-[calc(0.75rem+env(safe-area-inset-bottom,0px))]',
      )}
    >
      <a
        href={USER_MANUAL_URL}
        target="_blank"
        rel="noopener noreferrer"
        onClick={onNavigate}
        className="hover:bg-bg-elevated/50 mb-0.5 flex w-full items-center gap-2 rounded-lg px-2 py-2 transition-colors"
      >
        <BookOpen className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
        <span className="text-nav text-text-secondary min-w-0 flex-1 truncate">{t.navigation.userManual}</span>
      </a>
      <DropdownMenu>
        <UserMenuTrigger isMobile={isMobile} fullName={fullName} />
        <UserMenuItems
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
