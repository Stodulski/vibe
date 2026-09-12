import { Pencil, User } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { DEFAULT_PHONE_PREFIX } from '@/shared/lib/constants';
import { publicBookingSchema, type PublicBookingFormData } from '../../schemas/public-booking.schemas';
import type { SavedClientData } from './savedFormData';
import type { BookingPricing } from './pricing';
import { BookingSummaryCard } from './BookingSummaryCard';
import { SubmitFooter } from './SubmitFooter';
import { Panel } from '@/shared/components/common/Panel';
import type { BookingSlotInfo } from './types';

const t = ES_AR;

interface QuickBookViewProps {
  slotInfo: BookingSlotInfo;
  pricing: BookingPricing;
  saved: SavedClientData;
  isLoading: boolean;
  onSubmit: (data: PublicBookingFormData) => void;
  onEdit: () => void;
}

export function QuickBookView({ slotInfo, pricing, saved, isLoading, onSubmit, onEdit }: QuickBookViewProps) {
  const handleQuickSubmit = () => {
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

      <Panel size="sm" className="p-4">
        <div className="flex items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            <div className="bg-primary-500/10 flex size-10 shrink-0 items-center justify-center rounded-full">
              <User className="text-primary-400 size-4" />
            </div>
            <div className="min-w-0">
              <p className="text-text-primary truncate text-sm font-medium">
                {saved.client_first_name} {saved.client_last_name}
              </p>
              <p className="text-text-tertiary truncate text-xs">
                {DEFAULT_PHONE_PREFIX} {saved.client_phone}
                {saved.client_email ? ` · ${saved.client_email}` : ''}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onEdit}
            className="text-text-tertiary hover:bg-bg-elevated hover:text-text-secondary flex shrink-0 items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium transition-colors"
          >
            <Pencil className="size-3" />
            {t.common.change}
          </button>
        </div>
      </Panel>

      <SubmitFooter isLoading={isLoading} totalOnline={pricing.totalOnline} type="button" onClick={handleQuickSubmit} />
    </div>
  );
}
