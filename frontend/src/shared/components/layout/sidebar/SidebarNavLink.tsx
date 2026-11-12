import { NavLink } from 'react-router-dom';
import { cn } from '@/shared/lib/utils';

interface NavItem {
  to: string;
  label: string;
}

interface SidebarNavLinkProps {
  item: NavItem;
  isActive: boolean;
  isMobile: boolean;
  onNavigate?: (() => void) | undefined;
  onMouseEnter?: () => void;
}

/** Every width shows the label alone (owner request: name every part, no icons). */
export function SidebarNavLink({ item, isActive, isMobile, onNavigate, onMouseEnter }: SidebarNavLinkProps) {
  return (
    <NavLink
      to={item.to}
      onClick={onNavigate}
      onMouseEnter={onMouseEnter}
      aria-current={isActive ? 'page' : undefined}
      className={cn(
        'group press-scale relative flex items-center rounded-lg px-2.5 py-[7px] transition-colors duration-200',
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
      <span className="min-w-0 truncate">{item.label}</span>
    </NavLink>
  );
}
