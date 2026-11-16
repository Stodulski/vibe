import { Link } from 'react-router-dom';
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

/**
 * `Link`, not `NavLink`: the active state comes from the section the item owns
 * (`isActive`), which `NavLink` cannot know for Eventos (its link targets one tab).
 * Every width shows the label alone (owner request: name every part, no icons).
 */
export function SidebarNavLink({ item, isActive, isMobile, onNavigate, onMouseEnter }: SidebarNavLinkProps) {
  return (
    <Link
      to={item.to}
      onClick={onNavigate}
      onMouseEnter={onMouseEnter}
      aria-current={isActive ? 'page' : undefined}
      className={cn(
        'group press-scale flex items-center rounded-lg px-2.5 py-[7px] transition-colors duration-200',
        // The mobile sheet runs a notch smaller than the desktop rail (owner
        // request): 12px labels instead of the 13px `text-nav`.
        isMobile ? 'text-xs' : 'text-nav',
        // The selected part changes only its text colour (owner request): no
        // fill, no side bar, no weight change.
        isActive ? 'text-primary-500' : 'text-text-tertiary hover:text-text-secondary hover:bg-bg-elevated/40',
      )}
    >
      <span className="min-w-0 truncate">{item.label}</span>
    </Link>
  );
}
