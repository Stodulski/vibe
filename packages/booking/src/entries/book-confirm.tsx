import { BookingRoot } from '../BookingRoot';
import BookConfirmPage from '../pages/BookConfirmPage';
import type { BookingEntryProps } from './types';

export function BookConfirmEntry({ slug, config, initialSearch, toaster, children }: BookingEntryProps) {
  return (
    <BookingRoot config={config} initialSearch={initialSearch} toaster={toaster}>
      {children}
      <BookConfirmPage slug={slug} />
    </BookingRoot>
  );
}
