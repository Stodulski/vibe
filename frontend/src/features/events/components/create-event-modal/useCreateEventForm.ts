import { zodResolver } from '@hookform/resolvers/zod';
import { useAppForm } from '@/shared/lib/form';
import { pesosToCentavos } from '@/shared/lib/money';
import { useCreateEvent } from '../../hooks/useCreateEvent';
import { eventConflictMessage } from '../../lib/eventConflict';
import { eventFormSchema, type EventFormValues } from '../../schemas/eventForm.schema';
import type { Sport } from '@/shared/types/api.types';

const DEFAULTS: EventFormValues = {
  name: '',
  sport: 'padel',
  date: '',
  start_time: '',
  end_time: '',
  allCourts: true,
  courtIds: [],
  capacity: 20,
  price: Number.NaN,
};

/**
 * The create form and its submit. A 409 (something already holds the hours) is
 * kept as `conflict`, in Spanish, for the dialog to show beside the fields; any
 * other failure is the mutation's own toast-less error, shown generically.
 */
export function useCreateEventForm(complexId: string, onDone: () => void) {
  const createEvent = useCreateEvent(complexId);
  const form = useAppForm<EventFormValues>({
    resolver: zodResolver(eventFormSchema),
    defaultValues: DEFAULTS,
  });

  const onSubmit = (data: EventFormValues) => {
    createEvent.mutate(
      {
        name: data.name.trim(),
        sport: data.sport as Sport,
        date: data.date,
        start_time: data.start_time,
        end_time: data.end_time,
        // Absent means every active court.
        ...(data.allCourts ? {} : { court_ids: data.courtIds }),
        capacity: data.capacity,
        price: pesosToCentavos(data.price),
      },
      { onSuccess: onDone },
    );
  };

  return {
    form,
    onSubmit,
    isPending: createEvent.isPending,
    error: createEvent.error,
    conflict: eventConflictMessage(createEvent.error),
  };
}
