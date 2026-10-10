import { BookingRoot } from '../BookingRoot';
import BookCancelPage from '../pages/BookCancelPage';
import type { BookingEntryProps } from './types';

export function BookCancelEntry({ slug, config, toaster, children }: BookingEntryProps) {
  return (
    <BookingRoot config={config} toaster={toaster}>
      {children}
      <BookCancelPage slug={slug} />
    </BookingRoot>
  );
}
