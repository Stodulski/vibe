import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { instagramApi } from '../api/instagram.api';
import type { InstagramStoriesResponse, InstagramStorySettings } from '../schemas/instagramStories.schema';

const t = ES_AR.instagram.stories;

/**
 * The complex's automatic-story settings and its last stories. Only mounted
 * under a usable connection: the server answers 501 when Instagram is not
 * offered to this owner.
 */
export function useInstagramStories(complexId: string) {
  return useQuery({
    queryKey: queryKeys.complexes.instagramStories(complexId),
    queryFn: ({ signal }) => instagramApi.getStories(complexId, signal),
    // The section renders its own retry.
    throwOnError: false,
  });
}

/**
 * Saves the four settings. Field errors (422) are the form's to show; the
 * caller handles them in its own `onError`, so this toasts only on success.
 */
export function useUpdateInstagramStories(complexId: string) {
  const queryClient = useQueryClient();
  const queryKey = queryKeys.complexes.instagramStories(complexId);
  return useMutation({
    mutationFn: (settings: InstagramStorySettings) => instagramApi.updateStories(complexId, settings),
    onSuccess: ({ settings }) => {
      queryClient.setQueryData<InstagramStoriesResponse>(queryKey, (old) => (old ? { ...old, settings } : old));
      toast.success(t.saved);
    },
  });
}
