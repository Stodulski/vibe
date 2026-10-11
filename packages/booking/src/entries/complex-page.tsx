import { BookingRoot } from '../BookingRoot';
import ComplexPage from '../pages/ComplexPage';
import type { BookingEntryProps } from './types';

/** The storefront: the complex, its slot picker and the booking steps. */
export function ComplexPageEntry({
  slug,
  config,
  initialComplex,
  initialSearch,
  toaster,
  children,
}: BookingEntryProps) {
  return (
    <BookingRoot config={config} initialComplex={initialComplex} initialSearch={initialSearch} toaster={toaster}>
      {children}
      <ComplexPage slug={slug} />
    </BookingRoot>
  );
}
