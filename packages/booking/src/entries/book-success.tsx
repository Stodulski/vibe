import { BookingRoot } from '../BookingRoot';
import BookSuccessPage from '../pages/BookSuccessPage';
import type { BookingEntryProps } from './types';

export function BookSuccessEntry({ slug, config, toaster, children }: BookingEntryProps) {
  return (
    <BookingRoot config={config} toaster={toaster}>
      {children}
      <BookSuccessPage slug={slug} />
    </BookingRoot>
  );
}
