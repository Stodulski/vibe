import { useCallback } from 'react';
import { useAuth } from '@/features/auth/hooks/useAuth';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { ES_AR } from '@/shared/i18n/es_AR';
import { usePrefetch } from './usePrefetch';
import { cn } from '@/shared/lib/utils';
import { SidebarComplexHeader } from '@/shared/components/layout/sidebar/SidebarComplexHeader';
import { SidebarNav } from '@/shared/components/layout/sidebar/SidebarNav';
import { SidebarUserMenu } from './sidebar/SidebarUserMenu';

const t = ES_AR;

interface SidebarProps {
  onNavigate?: () => void;
  /** True when rendered inside the mobile nav sheet — shows labels and its own brand row. */
  isMobile?: boolean;
}

/** Desktop is permanently icon-only (no expand/collapse toggle); the mobile sheet stays fully expanded. */
export function Sidebar({ onNavigate, isMobile = false }: SidebarProps) {
  const { complex } = useSelectedComplex();
  const { user } = useAuth();
  const prefetch = usePrefetch(complex?.id ?? null);
  const handlePrefetch = useCallback(
    (to: string) => () => {
      prefetch(to);
    },
    [prefetch],
  );

  const collapsed = !isMobile;

  return (
    <aside
      className={cn(
        'flex h-full shrink-0 flex-col',
        // Fills its host on mobile rather than carrying a width of its own.
        // The sheet is 280px and this said 220, so the drawer opened with a
        // 60px band of empty background down its right edge — the panel's
        // width is the sheet's decision to make, not this component's.
        isMobile ? 'w-full' : 'border-border-subtle w-[68px] border-r',
      )}
      aria-label={t.layout.sidebar}
    >
      {complex && <SidebarComplexHeader complex={complex} collapsed={collapsed} isMobile={isMobile} />}

      <SidebarNav collapsed={collapsed} isMobile={isMobile} onNavigate={onNavigate} onPrefetch={handlePrefetch} />

      <SidebarUserMenu user={user} collapsed={collapsed} isMobile={isMobile} onNavigate={onNavigate} />
    </aside>
  );
}
