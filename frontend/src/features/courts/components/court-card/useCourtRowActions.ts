import { useCallback, useState } from 'react';
import { useUpdateCourt } from '../../hooks/useUpdateCourt';
import { useDeleteCourt } from '../../hooks/useDeleteCourt';
import type { CourtWithPrices } from '@/shared/types/api.types';

/**
 * The active toggle and the delete confirmation, shared by `CourtCard` and
 * `CourtTableRow` so a court's row has exactly one place that mutates it —
 * the card and the `xl` table are two renderings of the same row, not two
 * places that could drift on what "delete" or "toggle active" does.
 */
export function useCourtRowActions(court: CourtWithPrices, complexId: string) {
  const [deleteOpen, setDeleteOpen] = useState(false);
  const updateCourt = useUpdateCourt(complexId);
  const deleteCourt = useDeleteCourt(complexId);

  const handleToggleActive = useCallback(() => {
    updateCourt.mutate({
      courtId: court.id,
      data: { is_active: !court.is_active },
    });
  }, [updateCourt, court.id, court.is_active]);

  const handleDelete = useCallback(() => {
    deleteCourt.mutate(court.id, {
      onSuccess: () => {
        setDeleteOpen(false);
      },
    });
  }, [deleteCourt, court.id]);

  return {
    deleteOpen,
    setDeleteOpen,
    handleToggleActive,
    isTogglePending: updateCourt.isPending,
    handleDelete,
    isDeleting: deleteCourt.isPending,
  };
}
