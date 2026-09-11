import { useMemo, useState, type ReactNode } from 'react';
import { PageHeadingContext, type PageHeading } from './pageHeadingContextObject';

export function PageHeadingProvider({ children }: { children: ReactNode }) {
  const [heading, setHeading] = useState<PageHeading | null>(null);
  // Without the React Compiler (A2), a plain object literal here would get a
  // new identity on every render of this provider, re-rendering every
  // consumer even when `heading` itself hasn't changed.
  const value = useMemo(() => ({ heading, setHeading }), [heading]);

  return <PageHeadingContext.Provider value={value}>{children}</PageHeadingContext.Provider>;
}
