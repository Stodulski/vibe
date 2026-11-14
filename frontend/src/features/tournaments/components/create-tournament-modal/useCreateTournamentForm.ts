import { zodResolver } from '@hookform/resolvers/zod';
import { useFieldArray } from 'react-hook-form';
import { useAppForm } from '@/shared/lib/form';
import { useCreateTournament } from '../../hooks/useCreateTournament';
import {
  DEFAULT_CATEGORY,
  toCreateRequest,
  tournamentFormSchema,
  type TournamentFormValues,
} from '../../schemas/tournamentForm.schema';

const DEFAULTS: TournamentFormValues = {
  name: '',
  sport: 'padel',
  start_date: '',
  end_date: '',
  categories: [{ ...DEFAULT_CATEGORY }],
};

/** The create form (with its category rows) and its submit; the mutation's error is read by the dialog. */
export function useCreateTournamentForm(complexId: string, onDone: () => void) {
  const createTournament = useCreateTournament(complexId);
  const form = useAppForm<TournamentFormValues>({
    resolver: zodResolver(tournamentFormSchema),
    defaultValues: DEFAULTS,
  });
  const categories = useFieldArray({ control: form.control, name: 'categories' });

  const onSubmit = (data: TournamentFormValues) => {
    createTournament.mutate(toCreateRequest(data), { onSuccess: onDone });
  };

  return {
    form,
    categories,
    onSubmit,
    isPending: createTournament.isPending,
    error: createTournament.error,
    addCategory: () => {
      categories.append({ ...DEFAULT_CATEGORY });
    },
  };
}
