import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { captureException } from '@/shared/lib/observability';
import { uploadApi } from '../api/upload.api';
import { complexApi } from '../api/complex.api';
import { cropImage, LOGO_TARGET, COVER_TARGET, type CropArea } from '../lib/cropImage';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import type { HTTPError } from 'ky';

const t = ES_AR;

const ACCEPTED_TYPES = ['image/jpeg', 'image/png', 'image/webp'];
const MAX_INPUT_SIZE = 5 * 1024 * 1024; // 5MB

/**
 * Messages `mutationFn` below throws itself, already in Spanish and already
 * specific ("only JPG/PNG/WebP", "under 5MB") — worth showing as-is, unlike
 * an uncaught `TypeError` or a browser-internal `DOMException` (from
 * `compressImage`'s `createImageBitmap`, e.g.), whose raw `.message` is
 * neither translated nor something a person should have to read.
 */
const KNOWN_VALIDATION_MESSAGES = new Set<string>([t.complex.imageInvalidType, t.complex.imageTooLarge]);

interface UploadParams {
  file: File;
  type: 'logo' | 'cover';
  /** The part of `file` the owner chose, in the file's own pixels. */
  crop: CropArea;
}

/** The message for a file the upload would reject, or null when it is acceptable. */
export function validateImageFile(file: File): string | null {
  if (!ACCEPTED_TYPES.includes(file.type)) return t.complex.imageInvalidType;
  if (file.size > MAX_INPUT_SIZE) return t.complex.imageTooLarge;
  return null;
}

export function useUploadImage(complexId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ file, type, crop }: UploadParams) => {
      const invalid = validateImageFile(file);
      if (invalid) throw new Error(invalid);

      // Crop to the exact size and encode as WebP
      const target = type === 'logo' ? LOGO_TARGET : COVER_TARGET;
      const compressed = await cropImage(file, crop, target);

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
      if (KNOWN_VALIDATION_MESSAGES.has(error.message)) {
        toast.error(error.message);
        return;
      }
      toast.error(getHttpErrorMessage(error, t.complex.imageUploadError));
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
        captureException(err);
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
