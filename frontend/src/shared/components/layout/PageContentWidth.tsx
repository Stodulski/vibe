import type { ReactNode } from 'react';
import { cn } from '@/shared/lib/utils';

interface PageContentWidthProps {
  children: ReactNode;
  className?: string;
}

/**
 * Shared horizontal rhythm for the app's content column — caps width and
 * centers it. Used by both the masthead's title cell and `<main>` so they
 * always line up, instead of each hand-rolling its own `mx-auto max-w-*`.
 */
export function PageContentWidth({ children, className }: PageContentWidthProps) {
  return <div className={cn('mx-auto w-full max-w-[1600px]', className)}>{children}</div>;
}
