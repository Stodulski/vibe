import { BookingRoot } from '../BookingRoot';
import BookCancelPage from '../pages/BookCancelPage';
import type { BookingEntryProps } from './types';

export function BookCancelEntry({ slug, config, initialSearch, toaster, children }: BookingEntryProps) {
  return (
    <BookingRoot config={config} initialSearch={initialSearch} toaster={toaster}>
      {children}
      <BookCancelPage slug={slug} />
    </BookingRoot>
  );
}
