import { useState } from 'react';
import { DEFAULT_PHONE_PREFIX } from '@/shared/lib/constants';
import { publicBookingSchema, type PublicBookingFormData } from '../../schemas/public-booking.schema';
import type { SavedClientData } from './savedFormData';
import type { BookingPricing } from './pricing';
import { BookingSummaryCard } from './BookingSummaryCard';
import { SubmitFooter } from './SubmitFooter';
import { QuickBookIdentityConfirm } from './QuickBookIdentityConfirm';
import { QuickBookSavedIdentity } from './QuickBookSavedIdentity';
import { Panel } from '@/shared/components/common/Panel';
import type { BookingSlotInfo } from './types';

interface QuickBookViewProps {
  slotInfo: BookingSlotInfo;
  pricing: BookingPricing;
  saved: SavedClientData;
  isLoading: boolean;
  onSubmit: (data: PublicBookingFormData) => void;
  onEdit: () => void;
  /**
   * "No, soy otra persona" — the saved identity is not this visitor's.
   * `BookingForm` clears it from storage and resets the (shared) form to
   * empty before switching to `FullFormView`.
   */
  onNotMe: () => void;
}

export function QuickBookView({ slotInfo, pricing, saved, isLoading, onSubmit, onEdit, onNotMe }: QuickBookViewProps) {
  // A saved identity used to be submitted with one tap and no confirmation —
  // on a shared or public device, the next visitor's booking (and payment)
  // went out under whoever used it last. Requiring an explicit "Sí, soy yo"
  // first, with an equally prominent "No, soy otra persona" beside it, means
  // nothing from storage is ever submitted without this visitor confirming
  // it is actually them.
  const [confirmed, setConfirmed] = useState(false);

  const handleQuickSubmit = () => {
    if (!confirmed) return;
    // Argentina is the only prefix ever emitted now — a stale non-AR
    // `phone_prefix` from before the country selector was removed must not
    // be trusted (see savedFormData.ts).
    const fullPhone = DEFAULT_PHONE_PREFIX + (saved.client_phone ?? '');
    const data = {
      client_first_name: saved.client_first_name ?? '',
      client_last_name: saved.client_last_name ?? '',
      client_phone: fullPhone,
      client_email: saved.client_email ?? '',
      client_notes: '',
    };
    const result = publicBookingSchema.safeParse(data);
    if (result.success) {
      onSubmit(result.data);
    } else {
      onEdit();
    }
  };

  return (
    <div className="space-y-5">
      <BookingSummaryCard slotInfo={slotInfo} pricing={pricing} />

      <Panel size="sm" className="space-y-3 py-4 sm:p-4">
        <QuickBookSavedIdentity saved={saved} onEdit={onEdit} />

        {!confirmed && (
          <QuickBookIdentityConfirm
            onConfirm={() => {
              setConfirmed(true);
            }}
            onNotMe={onNotMe}
          />
        )}
      </Panel>

      {confirmed && (
        <SubmitFooter
          isLoading={isLoading}
          totalOnline={pricing.totalOnline}
          type="button"
          onClick={handleQuickSubmit}
        />
      )}
    </div>
  );
}
