import { TableCell, TableRow } from '@/shared/components/ui/table';
import { cn, formatMonthYear } from '@/shared/lib/utils';
import { ClientActionsMenu } from '../ClientActionsMenu';
import { AttendanceMeter } from '../client-card/AttendanceMeter';
import { attendancePct, attendanceTone } from '../client-card/attendance';
import type { Client } from '@/shared/types/api.types';

/**
 * One client, as a dense table row — the `xl` reading of `ClientCard`.
 *
 * The name is a real `<button>`, not the row itself, for the same reason
 * `TappableCard` never nests one interactive element inside another: the
 * actions menu is a sibling, not a descendant. It gives the row the same
 * single-tab-stop keyboard path the card has (focus the name, press
 * Enter/Space, the detail opens) and its accessible name is the client's own
 * name, exactly like the card's `buttonLabel`. The `<tr>`'s own `onClick` is
 * only a mouse convenience for the rest of the row; the actions cell stops
 * that click from bubbling so opening the menu never also opens the detail.
 *
 * No blocked badge here — the owner asked for none in the table, unlike the
 * card, which still dims and marks a blocked client with a `Ban` icon.
 *
 * Every cell stays on one line: a long value ends in an ellipsis (full text in
 * `title`) instead of wrapping and making its row taller than the rest.
 */
export function ClientTableRow({
  client,
  onSelect,
  onBlock,
}: {
  client: Client;
  onSelect: (client: Client) => void;
  onBlock: (client: Client) => void;
}) {
  const fullName = `${client.first_name} ${client.last_name}`;
  const pct = attendancePct(client);
  const tone = attendanceTone(pct);

  return (
    <TableRow
      onClick={() => {
        onSelect(client);
      }}
      className={cn('border-border-subtle hover:bg-bg-elevated cursor-pointer', client.is_blocked && 'opacity-60')}
    >
      <TableCell className="max-w-56">
        <button
          type="button"
          title={fullName}
          onClick={(e) => {
            e.stopPropagation();
            onSelect(client);
          }}
          className="text-text-primary focus-visible:ring-primary-400 block max-w-full truncate rounded text-left text-sm font-semibold hover:underline focus-visible:ring-2 focus-visible:outline-none"
        >
          {fullName}
        </button>
      </TableCell>

      <TableCell className="score-text text-text-tertiary max-w-44 truncate text-sm">{client.phone}</TableCell>

      <TableCell className="text-text-secondary max-w-48 truncate text-sm" title={client.email ?? undefined}>
        {client.email ?? '—'}
      </TableCell>

      <TableCell className="score-text text-text-primary text-right text-sm font-semibold">
        {client.total_bookings}
      </TableCell>

      <TableCell className="score-text text-text-primary text-right text-sm font-semibold">{client.no_shows}</TableCell>

      <TableCell className="text-right">
        <div className="ml-auto w-24">
          <AttendanceMeter pct={pct} tone={tone} />
        </div>
      </TableCell>

      <TableCell className="text-text-tertiary text-sm">{formatMonthYear(client.created_at)}</TableCell>

      <TableCell
        className="text-right"
        onClick={(e) => {
          e.stopPropagation();
        }}
      >
        <ClientActionsMenu client={client} onBlock={onBlock} />
      </TableCell>
    </TableRow>
  );
}
