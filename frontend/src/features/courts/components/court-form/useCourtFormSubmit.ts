import { useCreateCourt } from '../../hooks/useCreateCourt';
import { useUpdateCourt } from '../../hooks/useUpdateCourt';
import type { CreateCourtDto } from '../../schemas/courts.schema';
import type { CourtWithPrices } from '@/shared/types/api.types';

/** The form's `defaultValues` for a given court, or a blank court to create. */
export function courtDefaultValues(court: CourtWithPrices | undefined): CreateCourtDto {
  if (!court) {
    return { name: '', sport: 'padel', court_type: 'outdoor', description: '', has_lighting: false, surface: '' };
  }
  return {
    name: court.name,
    sport: court.sport,
    court_type: court.court_type,
    // The server omits the key when there is none; the form still needs a
    // string, and an empty one round-trips back to no description.
    description: court.description ?? '',
    has_lighting: court.has_lighting,
    // Same for the surface: absent means unspecified, which the select shows
    // as its empty option.
    surface: court.surface ?? '',
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
    const { surface, ...rest } = data;
    if (isEdit) {
      updateCourt.mutate(
        // "" tells the server to clear the stored surface; omitting the key
        // would leave it untouched.
        { courtId: court.id, data: { ...rest, surface: surface ?? '' } },
        {
          onSuccess: () => {
            onClose();
          },
        },
      );
    } else {
      // Nothing to clear on a new court: no surface is the absence of the key.
      createCourt.mutate(surface ? { ...rest, surface } : rest, {
        onSuccess: (result) => {
          onClose();
          onCreated?.({ ...result.court, prices: [] });
        },
      });
    }
  };

  return { isEdit, mutation, onSubmit };
}
