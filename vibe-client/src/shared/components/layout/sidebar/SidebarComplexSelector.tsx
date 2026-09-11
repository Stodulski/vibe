import { Building2, ChevronsUpDown } from 'lucide-react';
import { Tooltip, TooltipTrigger, TooltipContent } from '@/shared/components/ui/tooltip';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Complex } from '@/shared/types/api.types';

const t = ES_AR;

interface SidebarComplexSelectorProps {
  complex: Complex;
  collapsed: boolean;
  isMobile: boolean;
  onSelect: () => void;
}

export function SidebarComplexSelector({ complex, collapsed, isMobile, onSelect }: SidebarComplexSelectorProps) {
  return (
    <div className={cn('p-3', collapsed && !isMobile && 'p-2')}>
      {collapsed && !isMobile ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              onClick={onSelect}
              aria-label={`${t.complex.changeComplex}: ${complex.name}`}
              className="flex w-full items-center justify-center rounded-lg border border-border-subtle bg-bg-elevated/40 p-2.5 transition-colors hover:bg-bg-elevated/70"
            >
              <Building2 className="size-4 shrink-0 text-text-tertiary" aria-hidden="true" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="right" sideOffset={8}>
            {t.complex.changeComplex}: {complex.name}
          </TooltipContent>
        </Tooltip>
      ) : (
        <button
          type="button"
          onClick={onSelect}
          aria-label={`${t.complex.changeComplex}: ${complex.name}`}
          className="group flex w-full items-center gap-2.5 rounded-lg border border-border-subtle bg-bg-elevated/40 px-3 py-2.5 transition-colors hover:bg-bg-elevated/70 hover:border-border-default"
        >
          <Building2 className="size-4 shrink-0 text-text-tertiary" aria-hidden="true" />
          <span className="min-w-0 flex-1 truncate text-left text-sm font-medium text-text-primary">
            {complex.name}
          </span>
          <ChevronsUpDown className="size-3.5 shrink-0 text-text-tertiary" aria-hidden="true" />
        </button>
      )}
    </div>
  );
}
