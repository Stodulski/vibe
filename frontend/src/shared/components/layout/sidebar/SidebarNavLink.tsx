import { Link } from 'react-router-dom';
import { sidebarEntryClass } from './sidebarEntryClass';

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
      className={sidebarEntryClass(isActive, isMobile)}
    >
      <span className="min-w-0 truncate">{item.label}</span>
    </Link>
  );
}
