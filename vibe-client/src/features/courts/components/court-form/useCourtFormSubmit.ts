import { useCreateCourt } from '../../hooks/useCreateCourt';
import { useUpdateCourt } from '../../hooks/useUpdateCourt';
import type { CreateCourtDto } from '../../schemas/courts.schemas';
import type { CourtWithPrices } from '@/shared/types/api.types';

/** The form's `defaultValues` for a given court, or a blank court to create. */
export function courtDefaultValues(court: CourtWithPrices | undefined): CreateCourtDto {
  if (!court) {
    return { name: '', sport: 'padel', court_type: 'outdoor', description: '' };
  }
  return {
    name: court.name,
    sport: court.sport,
    court_type: court.court_type,
    // The server omits the key when there is none; the form still needs a
    // string, and an empty one round-trips back to no description.
    description: court.description ?? '',
  };
}

export function useCourtFormSubmit(
  complexId: string,
  court: CourtWithPrices | undefined,
  onClose: () => void,
  onCreated?: (court: CourtWithPrices) => void,
) {
  const isEdit = !!court;
  const createCourt = useCreateCourt(complexId);
  const updateCourt = useUpdateCourt(complexId);
  const mutation = isEdit ? updateCourt : createCourt;

  const onSubmit = (data: CreateCourtDto) => {
    if (isEdit) {
      updateCourt.mutate(
        { courtId: court.id, data },
        {
          onSuccess: () => {
            onClose();
          },
        },
      );
    } else {
      createCourt.mutate(data, {
        onSuccess: (result) => {
          onClose();
          onCreated?.({ ...result.court, prices: [] });
        },
      });
    }
  };

  return { isEdit, mutation, onSubmit };
}
