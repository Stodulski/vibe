import { createContext } from 'react';

export interface PageHeading {
  title: string;
}

export interface PageHeadingContextValue {
  heading: PageHeading | null;
  setHeading: (heading: PageHeading | null) => void;
}

export const PageHeadingContext = createContext<PageHeadingContextValue | null>(null);
