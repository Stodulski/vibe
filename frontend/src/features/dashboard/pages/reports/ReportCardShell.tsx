import type { ComponentProps } from 'react';
import { Panel } from '@/shared/components/common/Panel';

/** The panel container of `ReportCard` and the reports page skeleton: same Panel recipe, so they cannot drift. */
export function ReportCardShell(props: Omit<ComponentProps<typeof Panel>, 'as' | 'size'>) {
  return <Panel as="section" size="md" {...props} />;
}
