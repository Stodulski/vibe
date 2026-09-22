import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { complexApi } from '../api/complex.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

export function useDeleteComplex() {
  const t = ES_AR;
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  return useMutation({
    mutationFn: (complexId: string) => complexApi.delete(complexId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.all });
      toast.success(t.complex.deletedSuccess);
      // The account owns at most one complex — deleting it leaves none, so
      // there is nothing to fall back to but creating a new one.
      void navigate('/onboarding', { replace: true });
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.complex.deleteError));
    },
  });
}
