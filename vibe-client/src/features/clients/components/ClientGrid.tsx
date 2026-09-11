import { ClientCard } from './client-card/ClientCard';
import type { Client } from '@/shared/types/api.types';

/**
 * The clients, as cards.
 *
 * This replaced a TanStack table that was never a table: it held sorting state
 * but rendered no control to trigger it, so the only interaction it ever had
 * was the row click that opened the drawer. A grid of cards says the same
 * thing, reads the same on a phone and a desktop without a second mobile-only
 * component, and drops the machinery that was doing nothing.
 */
export function ClientGrid({
  clients,
  onSelectClient,
  onBlockClient,
}: {
  clients: Client[];
  onSelectClient: (client: Client) => void;
  onBlockClient: (client: Client) => void;
}) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      {clients.map((client) => (
        <ClientCard key={client.id} client={client} onSelect={onSelectClient} onBlock={onBlockClient} />
      ))}
    </div>
  );
}
