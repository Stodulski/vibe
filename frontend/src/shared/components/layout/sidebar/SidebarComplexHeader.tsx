import { Building2 } from 'lucide-react';
import { VisuallyHidden } from '@/shared/components/common/VisuallyHidden';
import { Tooltip, TooltipTrigger, TooltipContent } from '@/shared/components/ui/tooltip';
import { cn } from '@/shared/lib/utils';
import type { Complex } from '@/shared/types/api.types';

interface SidebarComplexHeaderProps {
  complex: Complex;
  collapsed: boolean;
  isMobile: boolean;
}

/**
 * The account owns at most one complex, so this is a static label, not a
 * switcher: no button, no click, no chevron. Collapsed desktop shows just
 * the icon, with the name in a tooltip.
 */
export function SidebarComplexHeader({ complex, collapsed, isMobile }: SidebarComplexHeaderProps) {
  return (
    <div className={cn('p-3', collapsed && !isMobile && 'p-2')}>
      {collapsed && !isMobile ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <div className="border-border-subtle bg-bg-elevated/40 flex w-full items-center justify-center rounded-lg border p-2.5">
              <Building2 className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
              <VisuallyHidden>{complex.name}</VisuallyHidden>
            </div>
          </TooltipTrigger>
          <TooltipContent side="right" sideOffset={8}>
            {complex.name}
          </TooltipContent>
        </Tooltip>
      ) : (
        <div className="border-border-subtle bg-bg-elevated/40 flex w-full items-center gap-2.5 rounded-lg border px-3 py-2.5">
          <Building2 className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
          <span className="text-text-primary min-w-0 flex-1 truncate text-left text-sm font-medium">
            {complex.name}
          </span>
        </div>
      )}
    </div>
  );
}
