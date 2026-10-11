import { Link } from 'react-router-dom';
import { cn } from '@/shared/lib/utils';
import { Logo } from '@/shared/components/common/Logo';
import type { ReactNode } from 'react';

interface AppHeaderProps {
  children?: ReactNode;
  className?: string;
}

export function AppHeader({ children, className }: AppHeaderProps) {
  return (
    <header
      className={cn(
        'border-border-subtle bg-bg-base safe-area-inset flex h-16 shrink-0 items-center justify-between border-b px-4 sm:px-6',
        className,
      )}
    >
      <Link to="/" className="flex items-center">
        <Logo />
      </Link>
      {children && <div className="flex items-center gap-2">{children}</div>}
    </header>
  );
}
