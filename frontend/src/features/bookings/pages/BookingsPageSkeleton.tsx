import { BookingCalendarSkeleton } from '../components/booking-calendar/BookingCalendarSkeleton';
import { BookingsPageHeaderSkeleton } from './bookings/BookingsPageHeaderSkeleton';
import { DateNavStripSkeleton } from './bookings/DateNavStripSkeleton';
import { DATE_NAV_SECTION_CLASS } from './bookings/bookingsLayout';

/** What `BookingsPage` draws while its route chunk loads: header, the date strip on its rule, then the calendar grid. */
export function BookingsPageSkeleton() {
  return (
    <div>
      <BookingsPageHeaderSkeleton />
      <div className={DATE_NAV_SECTION_CLASS}>
        <DateNavStripSkeleton />
      </div>
      <BookingCalendarSkeleton />
    </div>
  );
}
