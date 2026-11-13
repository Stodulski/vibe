import { useRef, useState } from 'react';
import { toast } from 'sonner';
import { useUploadImage, useDeleteImage, validateImageFile } from '../hooks/useUploadImage';
import { COVER_TARGET, LOGO_TARGET, type CropArea } from '../lib/cropImage';
import { ES_AR } from '@/shared/i18n/es_AR';

/**
 * Shared upload/delete state for a single image slot (logo or cover).
 *
 * Picking a file does not upload it: it becomes `pending`, which the slot
 * shows in a crop dialog. Only `confirmCrop` uploads, with the chosen area.
 */
export function useImageSlot(complexId: string, type: 'logo' | 'cover') {
  const fileRef = useRef<HTMLInputElement>(null);
  const [pending, setPending] = useState<{ file: File; url: string; small: boolean } | null>(null);
  const uploadMutation = useUploadImage(complexId);
  const deleteMutation = useDeleteImage(complexId);
  const isLoading = uploadMutation.isPending || deleteMutation.isPending;

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    const invalid = validateImageFile(file);
    if (invalid) {
      toast.error(invalid);
      return;
    }
    // Decode before opening the cropper: a file whose type says image but
    // whose bytes are not one would otherwise open an empty dialog.
    const url = URL.createObjectURL(file);
    const probe = new Image();
    probe.onload = () => {
      const target = type === 'logo' ? LOGO_TARGET : COVER_TARGET;
      const small = probe.naturalWidth < target.width || probe.naturalHeight < target.height;
      setPending({ file, url, small });
    };
    probe.onerror = () => {
      URL.revokeObjectURL(url);
      toast.error(ES_AR.complex.imageUnreadable);
    };
    probe.src = url;
  };

  const closeCrop = () => {
    if (pending) URL.revokeObjectURL(pending.url);
    setPending(null);
  };

  const confirmCrop = (crop: CropArea) => {
    if (pending) uploadMutation.mutate({ file: pending.file, type, crop });
    closeCrop();
  };

  return { fileRef, isLoading, handleFileChange, deleteMutation, pending, cancelCrop: closeCrop, confirmCrop };
}
