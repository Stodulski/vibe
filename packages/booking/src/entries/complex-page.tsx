import { BookingRoot } from '../BookingRoot';
import ComplexPage from '../pages/ComplexPage';
import type { BookingEntryProps } from './types';

/** The storefront: the complex, its slot picker and the booking steps. */
export function ComplexPageEntry({ slug, config, initialComplex, toaster, children }: BookingEntryProps) {
  return (
    <BookingRoot config={config} initialComplex={initialComplex} toaster={toaster}>
      {children}
      <ComplexPage slug={slug} />
    </BookingRoot>
  );
}
