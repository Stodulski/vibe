import { Link, useParams } from 'react-router-dom';
import { cn } from '@/shared/lib/utils';
import { Logo } from '@/shared/components/common/Logo';
import type { ReactNode } from 'react';

interface AppHeaderProps {
  children?: ReactNode;
  className?: string;
}

export function AppHeader({ children, className }: AppHeaderProps) {
  const { slug } = useParams<{ slug: string }>();

  return (
    <header
      className={cn(
        'flex h-16 shrink-0 items-center justify-between border-b border-border-subtle bg-bg-base px-4 sm:px-6 safe-area-inset',
        className,
      )}
    >
      <Link to={slug ? `/${slug}` : '/'} className="flex items-center">
        <Logo />
      </Link>
      {children && <div className="flex items-center gap-2">{children}</div>}
    </header>
  );
}
