import { BookingRoot } from '../BookingRoot';
import EventPage from '../pages/EventPage';
import type { BookingEntryProps } from './types';

/** The sign-up page of one open-play event, and where the checkout returns to. */
export function EventPageEntry({
  slug,
  eventId,
  config,
  initialSearch,
  toaster,
  children,
}: BookingEntryProps & { eventId: string }) {
  return (
    <BookingRoot config={config} initialSearch={initialSearch} toaster={toaster}>
      {children}
      <EventPage slug={slug} eventId={eventId} />
    </BookingRoot>
  );
}
