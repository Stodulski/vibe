import { useState, type ReactNode } from 'react';
import { PageHeadingContext, type PageHeading } from './pageHeadingContextObject';

export function PageHeadingProvider({ children }: { children: ReactNode }) {
  const [heading, setHeading] = useState<PageHeading | null>(null);
  const value = { heading, setHeading };

  return <PageHeadingContext.Provider value={value}>{children}</PageHeadingContext.Provider>;
}
