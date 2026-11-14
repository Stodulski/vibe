import { useState } from 'react';
import { useDeleteBlockedSlot } from '@/features/courts';
import type { BlockedSlot, FixedBlock } from '@/shared/types/api.types';

export function useBlockSlotSection(complexId: string) {
  const [blockOpen, setBlockOpen] = useState(false);
  const [selectedSlot, setSelectedSlot] = useState<BlockedSlot | null>(null);
  // A date a fixed weekly booking holds: pressed on the grid like a blocked slot, but handled by its series.
  const [selectedFixedBlock, setSelectedFixedBlock] = useState<FixedBlock | null>(null);
  // The mutation hook only builds a closure here; it's never actually
  // triggered before the `!state.selectedComplexId` guard in the page
  // returns `null` and no interactive element renders.
  const deleteBlockedSlot = useDeleteBlockedSlot(complexId);

  return {
    blockOpen,
    setBlockOpen,
    selectedSlot,
    setSelectedSlot,
    selectedFixedBlock,
    setSelectedFixedBlock,
    deleteBlockedSlot,
  };
}
