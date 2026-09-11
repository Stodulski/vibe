import { useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { createBookingSchema, type CreateBookingDto } from '../../schemas/booking.schemas';

/** Form instance + calendar popover state + watched field values. */
export function useBookingFormState() {
  const [calendarOpen, setCalendarOpen] = useState(false);

  const {
    register,
    handleSubmit,
    setValue,
    setError,
    reset,
    control,
    trigger,
    formState: { errors, dirtyFields },
  } = useForm<CreateBookingDto>({
    resolver: zodResolver(createBookingSchema),
  });

  const courtId = useWatch({ control, name: 'court_id' });
  const date = useWatch({ control, name: 'date' });
  const startTime = useWatch({ control, name: 'start_time' });
  const durationMinutes = useWatch({ control, name: 'duration_minutes' });
  const paymentOption = useWatch({ control, name: 'payment_option' });
  const depositAmount = useWatch({ control, name: 'deposit_amount' });
  const price = useWatch({ control, name: 'price' });

  return {
    register,
    handleSubmit,
    setValue,
    setError,
    reset,
    control,
    trigger,
    errors,
    dirtyFields,
    calendarOpen,
    setCalendarOpen,
    courtId,
    date,
    startTime,
    durationMinutes,
    paymentOption,
    depositAmount,
    price,
  };
}
