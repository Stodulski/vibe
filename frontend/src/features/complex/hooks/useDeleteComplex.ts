import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { complexApi } from '../api/complex.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { useStore } from '@/shared/stores';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

export function useDeleteComplex() {
  const t = ES_AR;
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const selectedComplexId = useStore((s) => s.selectedComplexId);
  const setSelectedComplexId = useStore((s) => s.setSelectedComplexId);

  return useMutation({
    mutationFn: (complexId: string) => complexApi.delete(complexId),
    onSuccess: (_data, complexId) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.all });
      if (selectedComplexId === complexId) {
        setSelectedComplexId(null);
      }
      toast.success(t.complex.deletedSuccess);
      void navigate('/complexes', { replace: true });
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.complex.deleteError));
    },
  });
}
