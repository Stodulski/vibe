import type { ComponentProps } from 'react';
import { Panel } from '@/shared/components/common/Panel';
import { cn } from '@/shared/lib/utils';

/** The card container of `PaymentOverview` and its skeleton: same Panel recipe, so they cannot drift. */
export function PaymentOverviewShell({ className, ...props }: ComponentProps<typeof Panel>) {
  return (
    <Panel as="section" size="sm" mobile="card" className={cn('flex h-full flex-col p-4', className)} {...props} />
  );
}
