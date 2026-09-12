import { useState } from 'react';

import { zodResolver } from '@hookform/resolvers/zod';
import { toast } from 'sonner';
import { createCourtSchema, type CreateCourtDto } from '@/features/courts';
import { useAppForm, submitHandler } from '@/shared/lib/form';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Court, CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR;

const DEFAULT_COURT_VALUES: CreateCourtDto = {
  name: '',
  sport: 'padel',
  court_type: 'outdoor',
  // Onboarding does not ask for a description. It is optional detail an owner
  // writes once the venue is set up, and the first-run flow is worth keeping
  // to the fields a court cannot exist without.
  description: '',
};

/** Minimal mutation shape -- avoids coupling to the concrete UseMutationResult generic params. */
export interface MutationLike<TData, TVariables> {
  mutate: (variables: TVariables, options?: { onSuccess?: (data: TData) => void; onError?: () => void }) => void;
  isPending: boolean;
}

export function useCourtSetupForm(createCourt: MutationLike<{ court: Court }, CreateCourtDto>) {
  const [pricingCourt, setPricingCourt] = useState<CourtWithPrices | null>(null);

  const courtForm = useAppForm<CreateCourtDto>({
    resolver: zodResolver(createCourtSchema),
    defaultValues: DEFAULT_COURT_VALUES,
  });

  const handleCourtSubmit = submitHandler(courtForm.handleSubmit, (data) => {
    createCourt.mutate(data, {
      onSuccess: (result) => {
        setPricingCourt({ ...result.court, prices: [] });
        courtForm.reset(DEFAULT_COURT_VALUES);
      },
      onError: () => {
        toast.error(t.courts.createError);
      },
    });
  });

  return { courtForm, handleCourtSubmit, pricingCourt, setPricingCourt };
}
