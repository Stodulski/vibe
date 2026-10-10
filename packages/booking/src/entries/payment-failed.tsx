import { BookingRoot } from '../BookingRoot';
import BookPage from '../pages/BookPage';
import type { BookingEntryProps } from './types';

/** MercadoPago's failure back_url. */
export function PaymentFailedEntry({ slug, config, initialSearch, toaster, children }: BookingEntryProps) {
  return (
    <BookingRoot config={config} initialSearch={initialSearch} toaster={toaster}>
      {children}
      <BookPage slug={slug} />
    </BookingRoot>
  );
}
