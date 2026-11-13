import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { cn } from '@/shared/lib/utils';
import { BACK_LINK_CLASS } from './backLinkLayout';

/** Back link of the cash session and product detail pages. */
export function BackLink({ to, children }: { to: string; children: ReactNode }) {
  return (
    <Link
      to={to}
      className={cn(BACK_LINK_CLASS, 'text-text-tertiary hover:text-text-secondary text-sm transition-colors')}
    >
      <ArrowLeft className="size-4" aria-hidden="true" />
      {children}
    </Link>
  );
}
