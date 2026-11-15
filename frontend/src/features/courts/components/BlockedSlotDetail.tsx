import { Sheet, SheetContent } from '@/shared/components/ui/sheet';
import { DetailSheetHeader } from '@/shared/components/common/DetailSheetHeader';
import { BlockedSlotDetailMenu } from './BlockedSlotDetailMenu';
import { InfoRow } from '@/shared/components/common/InfoRow';
import { useExitingValue } from '@/shared/hooks/useExitingValue';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDateFull } from '@/shared/lib/utils';
import { formatEventTimeRange } from '@/shared/lib/eventDates';
import type { BlockedSlot } from '@/shared/types/api.types';
import { keepTogether } from '@/shared/lib/keepTogether';

const t = ES_AR;

interface BlockedSlotDetailProps {
  open: boolean;
  onClose: () => void;
  slot: BlockedSlot | null;
  onDelete: (slotId: string) => void;
  isDeleting?: boolean;
}

/**
 * Built from the same pieces as the booking and client drawers —
 * `DetailSheetHeader`, bare `InfoRow`s, the destructive action in the header's
 * ⋯ menu (there is no positive action, so no footer) — rather
 * than its own header and its own boxed-icon rows. It was the only detail
 * sheet still carrying the icon treatment the others dropped.
 */
export function BlockedSlotDetail({ open, onClose, slot, onDelete, isDeleting }: BlockedSlotDetailProps) {
  // `open` here *is* `!!slot`, so closing clears the slot in the same tick and
  // a bare `if (!slot) return null` tore the sheet out before it could slide
  // away. Keep the last one on screen for the exit.
  const shown = useExitingValue(slot);
  if (!shown) return null;

  // `formatDateFull`, not a hand-rolled `new Date(date + 'T12:00:00')`: the
  // API sends a calendar date as `YYYY-MM-DDT00:00:00Z`, so appending a time
  // to it produced `...00:00:00ZT12:00:00` — an Invalid Date that threw a
  // RangeError out of `format` and took the whole panel down.
  const dateFormatted = formatDateFull(shown.date);

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <SheetContent side="right" className="sm:max-w-lg" showCloseButton={false}>
        <DetailSheetHeader
          title={t.bookings.blockedSlot}
          subtitle={<span className="first-letter:uppercase">{dateFormatted}</span>}
          menu={
            <BlockedSlotDetailMenu
              disabled={isDeleting}
              onUnblock={() => {
                onDelete(shown.id);
              }}
            />
          }
        />

        {/* Only the body scrolls; the header stays put. */}
        <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 pt-4 pb-8 sm:px-6">
          <InfoRow label={t.bookings.time}>
            <span className="score-text font-medium">{formatEventTimeRange(shown.start_time, shown.end_time)}</span>
          </InfoRow>
          <InfoRow label={t.bookings.court}>{keepTogether(shown.court_name ?? '')}</InfoRow>
          {shown.reason && (
            <InfoRow label={t.courts.blockReason}>
              <p className="break-words">{shown.reason}</p>
            </InfoRow>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
