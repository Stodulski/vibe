import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import * as Sentry from '@sentry/react';
import { uploadApi } from '../api/upload.api';
import { complexApi } from '../api/complex.api';
import { compressImage, LOGO_OPTIONS, COVER_OPTIONS } from '../utils/compressImage';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import type { HTTPError } from 'ky';

const t = ES_AR;

const ACCEPTED_TYPES = ['image/jpeg', 'image/png', 'image/webp'];
const MAX_INPUT_SIZE = 5 * 1024 * 1024; // 5MB

interface UploadParams {
  file: File;
  type: 'logo' | 'cover';
}

export function useUploadImage(complexId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ file, type }: UploadParams) => {
      // Validate input
      if (!ACCEPTED_TYPES.includes(file.type)) {
        throw new Error(t.complex.imageInvalidType);
      }
      if (file.size > MAX_INPUT_SIZE) {
        throw new Error(t.complex.imageTooLarge);
      }

      // Compress to WebP
      const options = type === 'logo' ? LOGO_OPTIONS : COVER_OPTIONS;
      const compressed = await compressImage(file, options);

      // Get presigned URL
      const { upload_url, public_url } = await uploadApi.presign(complexId, {
        type,
        content_type: 'image/webp',
        file_size: compressed.size,
      });

      // Upload to R2
      const uploadRes = await uploadApi.uploadToR2(upload_url, compressed);
      if (!uploadRes.ok) {
        throw new Error(t.complex.imageUploadError);
      }

      // Update complex
      const payload = type === 'logo' ? { logo_url: public_url } : { cover_url: public_url };
      return complexApi.update(complexId, payload);
    },
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.all });
      queryClient.setQueryData(queryKeys.complexes.detail(complexId), data);
      toast.success(t.complex.imageUploaded);
    },
    onError: (error: Error | HTTPError) => {
      if ('response' in error) {
        toast.error(getHttpErrorMessage(error, t.complex.imageUploadError));
      } else {
        toast.error(error.message || t.complex.imageUploadError);
      }
    },
  });
}

export function useDeleteImage(complexId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ type, currentUrl }: { type: 'logo' | 'cover'; currentUrl: string }) => {
      const payload = type === 'logo' ? { logo_url: '' } : { cover_url: '' };
      const result = await complexApi.update(complexId, payload);

      // Best effort R2 cleanup: the complex record is already updated, so a
      // failure here is not user-facing, but it must not vanish silently —
      // it means an orphaned file is left behind in R2.
      try {
        await uploadApi.deleteImage(complexId, currentUrl);
      } catch (err) {
        Sentry.captureException(err);
      }

      return result;
    },
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.all });
      queryClient.setQueryData(queryKeys.complexes.detail(complexId), data);
      toast.success(t.complex.imageDeleted);
    },
    onError: (error: Error | HTTPError) => {
      if ('response' in error) {
        toast.error(getHttpErrorMessage(error, t.complex.imageDeleteError));
      } else {
        toast.error(t.complex.imageDeleteError);
      }
    },
  });
}
