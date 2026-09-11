import { CourtField } from './CourtField';
import { TimeField } from './TimeField';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { CourtWithPrices } from '@/shared/types/api.types';
import type { CreateBookingDto } from '../../schemas/booking.schemas';

export function CourtAndTimeFields({
  activeCourts,
  courtId,
  selectedCourt,
  durationMinutes,
  startTime,
  date,
  timeSlots,
  setValue,
  errors,
}: {
  activeCourts: CourtWithPrices[];
  courtId: string | undefined;
  selectedCourt: CourtWithPrices | undefined;
  durationMinutes: number;
  startTime: string | undefined;
  date: string | undefined;
  timeSlots: string[];
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
}) {
  return (
    <>
      <CourtField
        activeCourts={activeCourts}
        courtId={courtId}
        selectedCourt={selectedCourt}
        durationMinutes={durationMinutes}
        setValue={setValue}
        errors={errors}
      />
      <TimeField
        courtId={courtId}
        date={date}
        startTime={startTime}
        timeSlots={timeSlots}
        setValue={setValue}
        errors={errors}
      />
    </>
  );
}
