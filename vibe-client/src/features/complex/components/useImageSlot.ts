import { useRef } from 'react';
import { useUploadImage, useDeleteImage } from '../hooks/useUploadImage';

/**
 * Shared upload/delete state for a single image slot (logo or cover).
 * Extracted from the identical logic duplicated in `LogoCard` and
 * `CoverCard` (slice 10, max-lines decomposition — also removes a DRY
 * violation per this repo's own principles).
 */
export function useImageSlot(complexId: string, type: 'logo' | 'cover') {
  const fileRef = useRef<HTMLInputElement>(null);
  const uploadMutation = useUploadImage(complexId);
  const deleteMutation = useDeleteImage(complexId);
  const isLoading = uploadMutation.isPending || deleteMutation.isPending;

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    uploadMutation.mutate({ file, type });
    e.target.value = '';
  };

  return { fileRef, isLoading, handleFileChange, deleteMutation };
}
