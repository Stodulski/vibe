import { useState, useMemo } from 'react';
import { useForm, type UseFormReturn, type UseFormWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { isToday } from 'date-fns/isToday';
import { parseISO } from 'date-fns/parseISO';
import { useBlockCourtSlot } from '../../hooks/useBlockCourtSlot';
import { useBlockedSlots } from '../../hooks/useBlockedSlots';
import { useBookingsByDate } from '@/shared/hooks/useBookingsByDate';
import { timeToMinutes, ALL_TIME_SLOTS } from './time-slots';
import { courtObstacles, windowIsOccupied } from '@/shared/lib/courtOccupancy';
import { blockSlotFormSchema, type BlockSlotFormValues } from '../../schemas/blockSlotForm.schema';
import type { Booking, BlockedSlot, CourtWithPrices } from '@/shared/types/api.types';

/** The grid these options come from steps every half hour. */
const SLOT_LENGTH_MINUTES = 30;

export interface BlockSlotPrefill {
  court_id?: string;
  date?: string;
  start_time?: string;
}

/** The form's `defaultValues`, from whatever the caller prefilled. */
export function blockSlotDefaultValues(prefill: BlockSlotPrefill | undefined): BlockSlotFormValues {
  return {
    court_id: prefill?.court_id ?? '',
    date: prefill?.date ?? '',
    start_time: prefill?.start_time ?? '',
    end_time: '',
    reason: '',
  };
}

function watchedFields(watch: UseFormWatch<BlockSlotFormValues>) {
  return {
    date: watch('date'),
    courtId: watch('court_id'),
    startTime: watch('start_time'),
    endTime: watch('end_time'),
    reason: watch('reason') ?? '',
  };
}

/**
 * Everything driven by `setValue`/`handleSubmit`: selecting a new date clears
 * the times it no longer applies to, picking a later start clears an end time
 * it would now precede, and submitting builds the wire payload from the form
 * shape. Pulled out of the hook body so `useBlockSlotForm` itself stays under
 * the function-length lint limit.
 */
function blockSlotActions(
  form: UseFormReturn<BlockSlotFormValues>,
  blockSlot: ReturnType<typeof useBlockCourtSlot>,
  onClose: () => void,
  endTime: string,
) {
  const { setValue, handleSubmit } = form;

  const handleSelectDate = (nextDate: string) => {
    setValue('date', nextDate, { shouldValidate: true });
    setValue('start_time', '', { shouldValidate: true });
    setValue('end_time', '', { shouldValidate: true });
  };

  const handleStartTimeChange = (time: string) => {
    setValue('start_time', time, { shouldValidate: true });
    if (endTime && endTime <= time) {
      setValue('end_time', '', { shouldValidate: true });
    }
  };

  const onSubmit = handleSubmit((data) => {
    // `BlockSlotRequest.reason?: string` is absent-or-present, not
    // present-with-`undefined` — only include it when there is one.
    const reason = data.reason === '' ? undefined : data.reason;
    blockSlot.mutate(
      {
        courtId: data.court_id,
        data: {
          date: data.date,
          start_time: data.start_time,
          end_time: data.end_time,
          ...(reason !== undefined ? { reason } : {}),
        },
      },
      { onSuccess: onClose },
    );
  });

  return {
    handleSelectDate,
    handleStartTimeChange,
    onSubmit,
    setCourtId: (id: string) => {
      setValue('court_id', id, { shouldValidate: true });
    },
    setEndTime: (time: string) => {
      setValue('end_time', time, { shouldValidate: true });
    },
    setReason: (value: string) => {
      setValue('reason', value);
    },
  };
}

function useSlotOptions(
  date: string,
  courtId: string,
  startTime: string,
  existingBookings: Booking[],
  existingBlocked: BlockedSlot[],
) {
  const startTimeOptions = useMemo(() => {
    let slots = ALL_TIME_SLOTS;

    // Filter past times for today.
    if (date && isToday(parseISO(date))) {
      const now = new Date();
      const currentTime = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`;
      slots = slots.filter((slot) => slot > currentTime);
    }

    // Filter occupied slots (bookings + existing blocks) for the selected court.
    //
    // Asked as an overlap between instants, through the same helper the booking
    // grid uses. This used to compare minutes of the day, and a booking running
    // 23:00 to 01:00 asked `slotMin >= 1380 && slotMin < 60` — a condition no
    // minute of any day satisfies. The booking became invisible here and every
    // hour it held was offered as free to block, so an owner could put a block
    // on top of a court that was already sold.
    if (courtId && date) {
      const obstacles = courtObstacles(existingBookings, existingBlocked, courtId, date);
      slots = slots.filter(
        (slotTime) => !windowIsOccupied(obstacles, date, timeToMinutes(slotTime), SLOT_LENGTH_MINUTES),
      );
    }

    return slots;
  }, [date, courtId, existingBookings, existingBlocked]);

  const endTimeOptions = useMemo(
    () => (startTime ? startTimeOptions.filter((slot) => slot > startTime) : startTimeOptions),
    [startTime, startTimeOptions],
  );

  return { startTimeOptions, endTimeOptions };
}

export function useBlockSlotForm(
  complexId: string,
  courts: CourtWithPrices[],
  prefill: BlockSlotPrefill | undefined,
  onClose: () => void,
) {
  const [calendarOpen, setCalendarOpen] = useState(false);
  const blockSlot = useBlockCourtSlot(complexId);
  const activeCourts = courts.filter((c) => c.is_active);

  const form = useForm<BlockSlotFormValues>({
    resolver: zodResolver(blockSlotFormSchema),
    mode: 'onChange',
    defaultValues: blockSlotDefaultValues(prefill),
  });
  const {
    control,
    watch,
    formState: { errors, isValid },
  } = form;
  const { date, courtId, startTime, endTime, reason } = watchedFields(watch);

  // Fetch existing bookings and blocked slots for the selected date.
  const { data: existingBookings = [] } = useBookingsByDate(date ? complexId : null, date);
  const { data: existingBlocked = [] } = useBlockedSlots(complexId, date, date);

  const { startTimeOptions, endTimeOptions } = useSlotOptions(
    date,
    courtId,
    startTime,
    existingBookings,
    existingBlocked,
  );
  const { handleSelectDate, handleStartTimeChange, setCourtId, setEndTime, setReason, onSubmit } = blockSlotActions(
    form,
    blockSlot,
    onClose,
    endTime,
  );

  return {
    control,
    errors,
    date,
    courtId,
    startTime,
    endTime,
    reason,
    setCourtId,
    setStartTime: handleStartTimeChange,
    setEndTime,
    setReason,
    calendarOpen,
    setCalendarOpen,
    activeCourts,
    startTimeOptions,
    endTimeOptions,
    handleClose: onClose,
    handleSelectDate,
    handleSubmit: onSubmit,
    isValid,
    isPending: blockSlot.isPending,
  };
}
