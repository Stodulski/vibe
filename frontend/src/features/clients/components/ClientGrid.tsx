import { useMediaQuery } from '@/shared/hooks/useMediaQuery';
import { ClientCard } from './client-card/ClientCard';
import { ClientTable } from './ClientTable';
import type { Client } from '@/shared/types/api.types';

/**
 * The clients, as cards below `xl` and as a dense table from `xl` up.
 *
 * The card grid replaced a TanStack table that was never a table: it held
 * sorting state but rendered no control to trigger it, so the only
 * interaction it ever had was the row click that opened the drawer. This
 * table is a real one — reused actions, same row-opens-the-drawer behaviour.
 *
 * `useMediaQuery` rather than a CSS `xl:hidden` pair mounting both trees: this
 * is "render a different component", exactly the case the hook's own doc
 * comment carves out from "style this differently" (a class). Mounting both
 * unconditionally would call `attendancePct` for every client twice — once
 * per card, once per table row — and defeat `ClientCard`'s `memo` on every
 * unrelated re-render, since a row it can never see would still be doing the
 * work `memo` exists to skip. `useSyncExternalStore` underneath means this is
 * already correct on the first paint, so nothing flashes the wrong layout.
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
  const isTableLayout = useMediaQuery('(min-width: 1280px)');

  if (isTableLayout) {
    return <ClientTable clients={clients} onSelectClient={onSelectClient} onBlockClient={onBlockClient} />;
  }

  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {clients.map((client) => (
        <ClientCard key={client.id} client={client} onSelect={onSelectClient} onBlock={onBlockClient} />
      ))}
    </div>
  );
}
