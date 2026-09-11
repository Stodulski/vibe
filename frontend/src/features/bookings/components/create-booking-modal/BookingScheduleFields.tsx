import { DateField } from './DateField';
import { CourtAndTimeFields } from './CourtAndTimeFields';
import { PriceOrManualPriceField } from './PriceOrManualPriceField';
import type { useCreateBookingForm } from './useCreateBookingForm';

/** Date, court, time, and price fields — the "when/where/how much" of a booking. */
export function BookingScheduleFields({ form }: { form: ReturnType<typeof useCreateBookingForm> }) {
  return (
    <>
      <DateField
        date={form.date}
        calendarOpen={form.calendarOpen}
        setCalendarOpen={form.setCalendarOpen}
        setValue={form.setValue}
        errors={form.errors}
      />
      <CourtAndTimeFields
        activeCourts={form.activeCourts}
        courtId={form.courtId}
        selectedCourt={form.selectedCourt}
        durationMinutes={form.durationMinutes}
        startTime={form.startTime}
        date={form.date}
        timeSlots={form.timeSlots}
        setValue={form.setValue}
        errors={form.errors}
      />
      <PriceOrManualPriceField
        estimatedPrice={form.estimatedPrice}
        priceRequired={form.priceRequired}
        price={form.price}
        setValue={form.setValue}
        errors={form.errors}
      />
    </>
  );
}
