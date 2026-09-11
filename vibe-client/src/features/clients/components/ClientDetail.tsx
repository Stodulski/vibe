import { Sheet, SheetContent } from '@/shared/components/ui/sheet';
import { useOutsideClickGrace } from '@/shared/hooks/useOutsideClickGrace';
import { useClient } from '../hooks/useClient';
import type { Client } from '@/shared/types/api.types';
import { ClientDetailHeader } from './client-detail/ClientDetailHeader';
import { ClientContactInfo } from './client-detail/ClientContactInfo';
import { ClientDetailStats } from './client-detail/ClientDetailStats';
import { ClientDetailNotes } from './client-detail/ClientDetailNotes';
import { ClientRecentBookings } from './client-detail/ClientRecentBookings';
import { BookingDetailModals } from '@/features/bookings';
import { getClientAttendance } from './client-detail/clientDetailUtils';
import { useClientNotes } from './client-detail/useClientNotes';
import { useClientBookingDetail } from '../hooks/useClientBookingDetail';

interface ClientDetailProps {
  open: boolean;
  onClose: () => void;
  client: Client | null;
  complexId: string;
  onBlock: (client: Client) => void;
}

export function ClientDetail({ open, onClose, client, complexId, onBlock }: ClientDetailProps) {
  const { data: detail } = useClient(complexId, open && client ? client.id : null);
  const onPointerDownOutside = useOutsideClickGrace(open);
  const c = detail?.client ?? client;
  const recentBookings = detail?.recent_bookings ?? [];
  const { notes, isSaving, saveError, onNotesChange, retrySave } = useClientNotes(complexId, c);
  const bookingDetail = useClientBookingDetail(complexId, c?.id ?? '');

  if (!c) return null;

  const fullName = `${c.first_name} ${c.last_name}`;
  const attendance = getClientAttendance(c);

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <SheetContent
        side="right"
        className="sm:max-w-lg"
        showCloseButton={false}
        onPointerDownOutside={onPointerDownOutside}
      >
        <ClientDetailHeader client={c} fullName={fullName} isBlocked={c.is_blocked} onBlock={onBlock} />

        <div className="flex min-h-0 flex-1 flex-col px-6 pt-2">
          <div className="shrink-0 space-y-6">
            <ClientContactInfo client={c} />
            <ClientDetailStats totalBookings={c.total_bookings} noShows={c.no_shows} attendance={attendance} />
            <ClientDetailNotes
              notes={notes}
              isSaving={isSaving}
              saveError={saveError}
              onChange={onNotesChange}
              onRetry={retrySave}
            />
          </div>
          <ClientRecentBookings
            bookings={recentBookings}
            onSelect={bookingDetail.handleSelectBooking}
            className="mt-6 min-h-0 flex-1"
          />
        </div>

        <BookingDetailModals detail={bookingDetail} complexId={complexId} hideClient />
      </SheetContent>
    </Sheet>
  );
}
