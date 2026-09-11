import type { ComponentType } from 'react';
import { NavLink } from 'react-router-dom';
import { Tooltip, TooltipTrigger, TooltipContent } from '@/shared/components/ui/tooltip';
import { cn } from '@/shared/lib/utils';

interface NavItem {
  to: string;
  label: string;
  icon: ComponentType<{ className?: string }>;
}

interface SidebarNavLinkProps {
  item: NavItem;
  isActive: boolean;
  collapsed: boolean;
  isMobile: boolean;
  onNavigate?: (() => void) | undefined;
  onMouseEnter?: () => void;
}

export function SidebarNavLink({ item, isActive, collapsed, isMobile, onNavigate, onMouseEnter }: SidebarNavLinkProps) {
  const link = (
    <NavLink
      to={item.to}
      onClick={onNavigate}
      onMouseEnter={onMouseEnter}
      aria-current={isActive ? 'page' : undefined}
      aria-label={item.label}
      className={cn(
        'group relative flex items-center rounded-lg text-nav transition-colors duration-200 press-scale',
        collapsed && !isMobile ? 'justify-center px-0 py-2.5' : 'gap-2.5 px-2.5 py-[7px]',
        isActive
          ? 'bg-primary-500/10 text-text-primary font-medium'
          : 'text-text-tertiary hover:text-text-secondary hover:bg-bg-elevated/40',
      )}
    >
      {isActive && (
        // Glow color reuses the existing --color-primary-500 token (CSS
        // relative color syntax) instead of hardcoding rgb(29,185,84) again.
        <span className="absolute left-0 top-1/2 -translate-y-1/2 h-4 w-[3px] rounded-r-full bg-primary-500 shadow-[0_0_8px_rgb(from_var(--color-primary-500)_r_g_b/30%)]" />
      )}
      <item.icon
        className={cn(
          'size-4 shrink-0 transition-colors duration-200',
          isActive ? 'text-primary-500' : 'text-text-tertiary group-hover:text-text-secondary',
        )}
        aria-hidden="true"
      />
      {(!collapsed || isMobile) && <span>{item.label}</span>}
    </NavLink>
  );

  if (collapsed && !isMobile) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>{link}</TooltipTrigger>
        <TooltipContent side="right" sideOffset={8}>
          {item.label}
        </TooltipContent>
      </Tooltip>
    );
  }

  return link;
}
