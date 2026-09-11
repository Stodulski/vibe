import { BlockSlotModal, BlockedSlotDetail } from '@/features/courts';
import type { BlockedSlot, CourtWithPrices } from '@/shared/types/api.types';

interface BlockSlotSectionProps {
  complexId: string;
  courts: CourtWithPrices[];
  blockOpen: boolean;
  onBlockClose: () => void;
  selectedSlot: BlockedSlot | null;
  onSelectedSlotClose: () => void;
  // A plain callback + a pending flag, not the mutation object itself — this
  // is a presentational component, and shouldn't need to know it's talking
  // to `useMutation` to be understood or tested.
  onDeleteSlot: (slotId: string, onSuccess: () => void) => void;
  isDeletingSlot: boolean;
}

export function BlockSlotSection({
  complexId,
  courts,
  blockOpen,
  onBlockClose,
  selectedSlot,
  onSelectedSlotClose,
  onDeleteSlot,
  isDeletingSlot,
}: BlockSlotSectionProps) {
  return (
    <>
      <BlockSlotModal open={blockOpen} onClose={onBlockClose} complexId={complexId} courts={courts} />
      <BlockedSlotDetail
        open={!!selectedSlot}
        onClose={onSelectedSlotClose}
        slot={selectedSlot}
        onDelete={(slotId) => {
          onDeleteSlot(slotId, onSelectedSlotClose);
        }}
        isDeleting={isDeletingSlot}
      />
    </>
  );
}
