import { useState } from 'react';
import { useWatch } from 'react-hook-form';
import { useAppForm } from '@/shared/lib/form';
import { zodResolver } from '@hookform/resolvers/zod';
import { createBookingSchema, type CreateBookingDto } from '../../schemas/booking.schema';

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
  } = useAppForm<CreateBookingDto>({
    resolver: zodResolver(createBookingSchema),
    // FORM-09: without this the inputs mount with `value === undefined`, so
    // React treats them as uncontrolled and then switches them to controlled on
    // the first keystroke — a dev warning, and a `reset()` that cannot put the
    // field back to a value it never had.
    // `useBookingReset` fills these in properly every time the modal opens;
    // this is what the fields hold before that first open.
    defaultValues: {
      court_id: '',
      date: '',
      start_time: '',
      client_first_name: '',
      client_last_name: '',
      client_phone: '',
      client_email: '',
      notes: '',
    },
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
