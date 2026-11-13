import type { ComponentProps } from 'react';
import { Panel } from '@/shared/components/common/Panel';
import { cn } from '@/shared/lib/utils';
import { EXPECTED_CASH_CARD_CLASS } from './cashLayout';

/** The card container of `ExpectedCashCard` and its skeleton: same Panel recipe, so they cannot drift. */
export function ExpectedCashCardShell({ className, ...props }: ComponentProps<typeof Panel>) {
  return <Panel size="sm" mobile="card" className={cn(EXPECTED_CASH_CARD_CLASS, className)} {...props} />;
}
