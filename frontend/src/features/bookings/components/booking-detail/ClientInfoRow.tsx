import { ChevronRight } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Booking, Client } from '@/shared/types/api.types';

const t = ES_AR;

interface ClientInfoRowProps {
  booking: Booking;
  client: Client | undefined;
  /** Opens the client's own drawer; without it the row is plain text. */
  onOpenClient?: ((client: Client) => void) | undefined;
}

/**
 * The client as one more row of the booking's details, rather than a section
 * of its own: the name is all this sheet needs to say, and the rest lives one
 * tap away in the client's drawer.
 */
export function ClientInfoRow({ booking, client, onOpenClient }: ClientInfoRowProps) {
  const name = client ? `${client.first_name} ${client.last_name}` : booking.client_name;

  const label = <p className="text-text-tertiary text-xs font-medium whitespace-nowrap">{t.bookings.client}</p>;

  if (!client || !onOpenClient) {
    return (
      <div>
        {label}
        <p className="text-text-primary mt-0.5 text-sm font-medium">{name}</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-start">
      {label}
      <button
        type="button"
        onClick={() => {
          onOpenClient(client);
        }}
        // Hugs its content so the chevron sits right after the name instead of
        // drifting to the far edge of the sheet.
        className="hover:bg-bg-base/40 -mx-2 mt-0.5 flex max-w-full items-center gap-1 rounded-lg px-2 py-1 text-left transition-colors"
      >
        <span className="text-text-primary truncate text-sm font-medium">{name}</span>
        <ChevronRight className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
      </button>
    </div>
  );
}
