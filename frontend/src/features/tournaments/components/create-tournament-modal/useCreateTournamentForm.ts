import { type SyntheticEvent } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useFieldArray, type FieldErrors } from 'react-hook-form';
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
  categories: [],
};

interface SubmitHandlers {
  /** The server refused the tournament: the dialog goes to the step the error names. */
  onError: (error: unknown) => void;
  /** The form itself is invalid (a step was skipped): the dialog goes to the step that has the errors. */
  onInvalid: (errors: FieldErrors<TournamentFormValues>) => void;
}

/** The create form (with its category rows) and its submit; the mutation's error is read by the dialog. */
export function useCreateTournamentForm(complexId: string, onDone: () => void) {
  const createTournament = useCreateTournament(complexId);
  const form = useAppForm<TournamentFormValues>({
    resolver: zodResolver(tournamentFormSchema),
    defaultValues: DEFAULTS,
  });
  const categories = useFieldArray({ control: form.control, name: 'categories' });

  const submit =
    ({ onError, onInvalid }: SubmitHandlers) =>
    (event: SyntheticEvent<HTMLFormElement>) => {
      event.preventDefault();
      const onValid = (data: TournamentFormValues) => {
        createTournament.mutate(toCreateRequest(data), { onSuccess: onDone, onError });
      };
      void form.handleSubmit(onValid, onInvalid)(event);
    };

  return {
    form,
    categories,
    submit,
    isPending: createTournament.isPending,
    error: createTournament.error,
    clearError: createTournament.reset,
    /** Appends a blank category and returns its position. */
    addCategory: () => {
      categories.append({ ...DEFAULT_CATEGORY });
      return categories.fields.length;
    },
    removeCategory: (index: number) => {
      categories.remove(index);
    },
  };
}

export type CreateTournamentForm = ReturnType<typeof useCreateTournamentForm>;
