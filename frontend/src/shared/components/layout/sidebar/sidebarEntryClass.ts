import { cn } from '@/shared/lib/utils';

/** The classes a sidebar entry wears, shared by the in-app links and the external manual link. */
export function sidebarEntryClass(isActive: boolean, isMobile: boolean): string {
  return cn(
    'group press-scale flex items-center rounded-lg px-2.5 py-[7px] transition-colors duration-200',
    isMobile ? 'text-xs' : 'text-nav',
    isActive ? 'text-primary-500' : 'text-text-tertiary hover:text-text-secondary hover:bg-bg-elevated/40',
  );
}
