import type { ComponentType } from 'react';
import { NavLink } from 'react-router-dom';
import { cn } from '@/shared/lib/utils';

interface NavItem {
  to: string;
  label: string;
  icon: ComponentType<{ className?: string }>;
}

interface SidebarNavLinkProps {
  item: NavItem;
  isActive: boolean;
  isMobile: boolean;
  onNavigate?: (() => void) | undefined;
  onMouseEnter?: () => void;
}

/**
 * Desktop shows the label alone (owner request: name every part, no icons, to
 * save space); the mobile sheet keeps the icon beside the label.
 */
export function SidebarNavLink({ item, isActive, isMobile, onNavigate, onMouseEnter }: SidebarNavLinkProps) {
  return (
    <NavLink
      to={item.to}
      onClick={onNavigate}
      onMouseEnter={onMouseEnter}
      aria-current={isActive ? 'page' : undefined}
      className={cn(
        'group press-scale relative flex items-center gap-2.5 rounded-lg px-2.5 py-[7px] transition-colors duration-200',
        // The mobile sheet runs a notch smaller than the desktop rail (owner
        // request): 12px labels instead of the 13px `text-nav`.
        isMobile ? 'text-xs' : 'text-nav',
        isActive
          ? 'bg-primary-500/10 text-text-primary font-medium'
          : 'text-text-tertiary hover:text-text-secondary hover:bg-bg-elevated/40',
      )}
    >
      {isActive && (
        // Glow color reuses the existing --color-primary-500 token (CSS
        // relative color syntax) instead of hardcoding rgb(29,185,84) again.
        <span className="bg-primary-500 absolute top-1/2 left-0 h-4 w-[3px] -translate-y-1/2 rounded-r-full shadow-[0_0_8px_rgb(from_var(--color-primary-500)_r_g_b/30%)]" />
      )}
      {isMobile && (
        <item.icon
          className={cn(
            'size-3.5 shrink-0 transition-colors duration-200',
            isActive ? 'text-primary-500' : 'text-text-tertiary group-hover:text-text-secondary',
          )}
          aria-hidden="true"
        />
      )}
      <span className="min-w-0 truncate">{item.label}</span>
    </NavLink>
  );
}
