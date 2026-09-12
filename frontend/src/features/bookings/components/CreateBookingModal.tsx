import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CreateBookingSteps } from './create-booking-modal/CreateBookingSteps';
import { useCreateBookingForm } from './create-booking-modal/useCreateBookingForm';
import type { CreateBookingPrefill } from './create-booking-modal/useBookingReset';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

const t = ES_AR;

interface CreateBookingModalProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  courts: CourtWithPrices[];
  depositPercentage?: number;
  /** Opening hours, so the price preview charges from the window the hour came out of. */
  schedules: Schedule[];
  prefill?: CreateBookingPrefill | undefined;
}

export function CreateBookingModal({
  open,
  onClose,
  complexId,
  courts,
  depositPercentage = 0,
  schedules,
  prefill,
}: CreateBookingModalProps) {
  const form = useCreateBookingForm({
    open,
    complexId,
    courts,
    depositPercentage,
    prefill,
    schedules,
  });

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t.bookings.create}</DialogTitle>
        </DialogHeader>

        <CreateBookingSteps form={form} onClose={onClose} />
      </DialogContent>
    </Dialog>
  );
}
