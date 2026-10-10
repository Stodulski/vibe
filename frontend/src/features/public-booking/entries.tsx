import type { ReactNode } from 'react';
import { BookingRoot, type BookingRootProps } from './BookingRoot';
import type { BookingConfig } from './config';
import ComplexPage from './pages/ComplexPage';
import BookPage from './pages/BookPage';
import BookConfirmPage from './pages/BookConfirmPage';
import BookSuccessPage from './pages/BookSuccessPage';
import BookCancelPage from './pages/BookCancelPage';

/**
 * One entry per booking page. Each one mounts the page inside its own
 * `BookingRoot`, so a host renders a single component and gets the providers,
 * the config and the endpoints with it.
 */
export interface BookingEntryProps {
  slug: string;
  config: BookingConfig;
  /** Seeds the complex query. Only the complex page reads it. */
  initialComplex?: BookingRootProps['initialComplex'];
  /** Mount a Toaster. Pass `false` where the host app already mounts one. */
  toaster?: boolean;
  /** Rendered inside the providers, before the page: head tags and other host wiring. */
  children?: ReactNode;
}

export function ComplexPageEntry({ slug, config, initialComplex, toaster, children }: BookingEntryProps) {
  return (
    <BookingRoot config={config} initialComplex={initialComplex} toaster={toaster}>
      {children}
      <ComplexPage slug={slug} />
    </BookingRoot>
  );
}

export function PaymentFailedEntry({ slug, config, toaster, children }: BookingEntryProps) {
  return (
    <BookingRoot config={config} toaster={toaster}>
      {children}
      <BookPage slug={slug} />
    </BookingRoot>
  );
}

export function BookConfirmEntry({ slug, config, toaster, children }: BookingEntryProps) {
  return (
    <BookingRoot config={config} toaster={toaster}>
      {children}
      <BookConfirmPage slug={slug} />
    </BookingRoot>
  );
}

export function BookSuccessEntry({ slug, config, toaster, children }: BookingEntryProps) {
  return (
    <BookingRoot config={config} toaster={toaster}>
      {children}
      <BookSuccessPage slug={slug} />
    </BookingRoot>
  );
}

export function BookCancelEntry({ slug, config, toaster, children }: BookingEntryProps) {
  return (
    <BookingRoot config={config} toaster={toaster}>
      {children}
      <BookCancelPage slug={slug} />
    </BookingRoot>
  );
}
