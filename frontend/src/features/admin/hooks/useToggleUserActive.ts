import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { queryKeys } from '@/shared/lib/queryKeys';
import { adminApi } from '../api/admin.api';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function useToggleUserActive() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ userId, isActive }: { userId: string; isActive: boolean }) =>
      adminApi.toggleUserActive(userId, isActive),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.admin.usersBase });
      void queryClient.invalidateQueries({ queryKey: queryKeys.admin.stats });
      toast.success(variables.isActive ? t.admin.users.activated : t.admin.users.deactivated);
    },
    onError: () => {
      toast.error(t.admin.users.toggleError);
    },
  });
}
