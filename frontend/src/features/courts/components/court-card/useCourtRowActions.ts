import { useCallback, useState } from 'react';
import { useUpdateCourt } from '../../hooks/useUpdateCourt';
import { useDeleteCourt } from '../../hooks/useDeleteCourt';
import type { CourtWithPrices } from '@/shared/types/api.types';

/**
 * The active toggle and its two confirmations, shared by `CourtCard` and
 * `CourtTableRow` so a court's row has exactly one place that mutates it —
 * the card and the `xl` table are two renderings of the same row, not two
 * places that could drift on what "delete" or "toggle active" does.
 */
export function useCourtRowActions(court: CourtWithPrices, complexId: string) {
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deactivateOpen, setDeactivateOpen] = useState(false);
  const updateCourt = useUpdateCourt(complexId);
  const deleteCourt = useDeleteCourt(complexId);

  // Turning a court off takes it out of online booking and the grid, so it asks first;
  // turning it back on is harmless and goes straight through.
  const handleToggleActive = useCallback(() => {
    if (court.is_active) {
      setDeactivateOpen(true);
      return;
    }
    updateCourt.mutate({ courtId: court.id, data: { is_active: true } });
  }, [updateCourt, court.id, court.is_active]);

  const handleConfirmDeactivate = useCallback(() => {
    updateCourt.mutate(
      { courtId: court.id, data: { is_active: false } },
      {
        onSettled: () => {
          setDeactivateOpen(false);
        },
      },
    );
  }, [updateCourt, court.id]);

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
    deactivateOpen,
    setDeactivateOpen,
    handleConfirmDeactivate,
    isTogglePending: updateCourt.isPending,
    handleDelete,
    isDeleting: deleteCourt.isPending,
  };
}
