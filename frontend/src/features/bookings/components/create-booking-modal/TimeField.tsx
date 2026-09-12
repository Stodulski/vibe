import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schema';

const t = ES_AR;

function timePlaceholder(courtId: string | undefined, date: string | undefined, timeSlots: string[]) {
  if (!courtId && !date) return t.bookings.selectDateAndCourt;
  if (!date) return t.bookings.selectDateFirst;
  if (!courtId) return t.bookings.selectCourt;
  if (timeSlots.length === 0) return t.publicBooking.noAvailableSlots;
  return '--:--';
}

export function TimeField({
  courtId,
  date,
  startTime,
  timeSlots,
  setValue,
  errors,
}: {
  courtId: string | undefined;
  date: string | undefined;
  startTime: string | undefined;
  timeSlots: string[];
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
}) {
  return (
    <FormField
      label={
        <>
          {t.bookings.time}
          <FieldRequirement required />
        </>
      }
      htmlFor="booking-start-time"
      error={errors.start_time?.message}
    >
      <Select
        {...(startTime !== undefined ? { value: startTime } : {})}
        onValueChange={(v) => {
          setValue('start_time', v);
        }}
        disabled={!courtId || !date || timeSlots.length === 0}
      >
        <SelectTrigger id="booking-start-time">
          <SelectValue placeholder={timePlaceholder(courtId, date, timeSlots)} />
        </SelectTrigger>
        <SelectContent>
          {timeSlots.map((slot) => (
            <SelectItem key={slot} value={slot}>
              {slot}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FormField>
  );
}
