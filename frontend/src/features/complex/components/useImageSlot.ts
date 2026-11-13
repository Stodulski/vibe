import { useRef, useState } from 'react';
import { toast } from 'sonner';
import { useUploadImage, useDeleteImage, validateImageFile } from '../hooks/useUploadImage';
import type { CropArea } from '../lib/cropImage';

/**
 * Shared upload/delete state for a single image slot (logo or cover).
 *
 * Picking a file does not upload it: it becomes `pending`, which the slot
 * shows in a crop dialog. Only `confirmCrop` uploads, with the chosen area.
 */
export function useImageSlot(complexId: string, type: 'logo' | 'cover') {
  const fileRef = useRef<HTMLInputElement>(null);
  const [pending, setPending] = useState<{ file: File; url: string } | null>(null);
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
    setPending({ file, url: URL.createObjectURL(file) });
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
