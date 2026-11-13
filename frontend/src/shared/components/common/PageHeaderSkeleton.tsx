import type { ReactNode } from 'react';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { cn } from '@/shared/lib/utils';
import { PAGE_HEADER_CLASS } from './pageHeaderLayout';

interface PageHeaderSkeletonProps {
  /** Width class of the title bar. */
  titleWidth?: string;
  /** Action placeholders beside the title, for pages whose header has a button. */
  actions?: ReactNode;
}

/**
 * Loading stand-in for `PageHeader`: the title is mobile-only (the masthead
 * carries it from `md`), so without actions this is just that one bar.
 */
export function PageHeaderSkeleton({ titleWidth = 'w-32', actions }: PageHeaderSkeletonProps) {
  if (!actions) return <Skeleton className={cn('mb-3 h-7 rounded-lg sm:mb-5 md:hidden', titleWidth)} />;

  return (
    <div className={PAGE_HEADER_CLASS}>
      <Skeleton className={cn('h-7 rounded-lg md:hidden', titleWidth)} />
      {actions}
    </div>
  );
}
