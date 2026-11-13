import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { TABLE_CELL_PADDING_CLASS, TABLE_FRAME_CLASS } from '@/shared/components/common/tableFrameLayout';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { ClientTableRow } from './client-table/ClientTableRow';
import type { Client } from '@/shared/types/api.types';

const t = ES_AR;

const HEAD_CELL = 'text-micro text-text-tertiary h-9 font-medium tracking-wider whitespace-nowrap uppercase';

/**
 * The clients, as a dense table — the `xl`-and-up reading of `ClientGrid`'s
 * cards. Same data, same actions, same row-opens-the-detail behaviour; only
 * the shape changes once there's room for one row per client instead of a
 * few cards per line.
 */
export function ClientTable({
  clients,
  onSelectClient,
  onBlockClient,
}: {
  clients: Client[];
  onSelectClient: (client: Client) => void;
  onBlockClient: (client: Client) => void;
}) {
  // The cells carry the table's own side padding: `TableCell`'s default `p-2`
  // leaves the first and last columns almost touching the rounded border,
  // which reads as text falling out of the panel.
  return (
    <div className={cn(TABLE_FRAME_CLASS, TABLE_CELL_PADDING_CLASS)}>
      <Table>
        <TableHeader className="bg-bg-elevated">
          <TableRow className="border-border-subtle hover:bg-transparent">
            <TableHead className={HEAD_CELL}>{t.clients.table.client}</TableHead>
            <TableHead className={HEAD_CELL}>{t.clients.phone}</TableHead>
            <TableHead className={HEAD_CELL}>{t.clients.email}</TableHead>
            <TableHead className={`${HEAD_CELL} text-right`}>{t.clients.bookingsLabel}</TableHead>
            <TableHead className={`${HEAD_CELL} text-right`}>{t.clients.noShows}</TableHead>
            <TableHead className={`${HEAD_CELL} text-right`}>{t.clients.attendanceLabel}</TableHead>
            <TableHead className={HEAD_CELL}>{t.clients.table.since}</TableHead>
            <TableHead className={`${HEAD_CELL} text-right`}>{t.common.actions}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody className="whitespace-nowrap">
          {clients.map((client) => (
            <ClientTableRow key={client.id} client={client} onSelect={onSelectClient} onBlock={onBlockClient} />
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
