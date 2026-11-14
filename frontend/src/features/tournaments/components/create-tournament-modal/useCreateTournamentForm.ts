import { useState, type SyntheticEvent } from 'react';
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

  // The accordion items are told apart by their position; the first category starts open.
  const [open, setOpen] = useState<string[]>(['0']);

  const onValid = (data: TournamentFormValues) => {
    createTournament.mutate(toCreateRequest(data), { onSuccess: onDone });
  };
  // A category folded away can hold the error: open every one that does, so it is not hidden.
  const onInvalid = (errors: FieldErrors<TournamentFormValues>) => {
    const withErrors = Object.keys(errors.categories ?? {}).filter((key) => /^\d+$/.test(key));
    setOpen((prev) => [...new Set([...prev, ...withErrors])]);
  };
  const onSubmit = (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    void form.handleSubmit(onValid, onInvalid)(event);
  };

  return {
    form,
    categories,
    onSubmit,
    open,
    setOpen,
    isPending: createTournament.isPending,
    error: createTournament.error,
    /** A new category opens and the others fold, so the dialog stays short. */
    addCategory: () => {
      setOpen([String(categories.fields.length)]);
      categories.append({ ...DEFAULT_CATEGORY });
    },
    removeCategory: (index: number) => {
      categories.remove(index);
      setOpen((prev) =>
        prev.filter((key) => key !== String(index)).map((key) => (Number(key) > index ? String(Number(key) - 1) : key)),
      );
    },
  };
}
