import { Building2 } from 'lucide-react';
import type { Complex } from '@/shared/types/api.types';

interface SidebarComplexHeaderProps {
  complex: Complex;
  isMobile: boolean;
}

/**
 * The account owns at most one complex, so this is a static label, not a
 * switcher: no button, no click, no chevron. Desktop shows the name alone;
 * the mobile sheet keeps the building icon beside it.
 */
export function SidebarComplexHeader({ complex, isMobile }: SidebarComplexHeaderProps) {
  return (
    <div className="p-3">
      <div className="border-border-subtle bg-bg-elevated/40 flex w-full items-center gap-2.5 rounded-lg border px-3 py-2.5">
        {isMobile && <Building2 className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />}
        <span className="text-text-primary min-w-0 flex-1 truncate text-left text-sm font-medium">{complex.name}</span>
      </div>
    </div>
  );
}
