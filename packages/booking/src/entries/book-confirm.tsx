import { BookingRoot } from '../BookingRoot';
import BookConfirmPage from '../pages/BookConfirmPage';
import type { BookingEntryProps } from './types';

export function BookConfirmEntry({ slug, config, toaster, children }: BookingEntryProps) {
  return (
    <BookingRoot config={config} toaster={toaster}>
      {children}
      <BookConfirmPage slug={slug} />
    </BookingRoot>
  );
}
