import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CourtTableRow } from './court-table/CourtTableRow';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR;

const HEAD_CELL = 'text-micro text-text-tertiary h-9 font-medium tracking-wider whitespace-nowrap uppercase';

interface CourtTableProps {
  courts: CourtWithPrices[];
  complexId: string;
  onEdit: (court: CourtWithPrices) => void;
  onPrices: (court: CourtWithPrices) => void;
}

/**
 * The courts, as a dense table — the `xl`-and-up reading of `CourtGrid`'s
 * cards. Below `xl` there is room for a handful of wide cards a column or
 * two deep; at `xl` and up the grid would run three cards to a row with a
 * lot of empty card padding doing nothing, where a bordered table reads a
 * dozen courts at once.
 */
export function CourtTable({ courts, complexId, onEdit, onPrices }: CourtTableProps) {
  return (
    <div className="border-border-default bg-bg-subtle w-full overflow-hidden rounded-2xl border">
      <Table>
        <TableHeader className="bg-bg-elevated">
          <TableRow className="border-border-subtle hover:bg-transparent">
            <TableHead className={HEAD_CELL}>{t.courts.table.court}</TableHead>
            <TableHead className={HEAD_CELL}>{t.courts.table.type}</TableHead>
            <TableHead className={HEAD_CELL}>{t.courts.table.description}</TableHead>
            <TableHead className={`${HEAD_CELL} text-right`}>{t.courts.price}</TableHead>
            <TableHead className={HEAD_CELL}>{t.courts.table.status}</TableHead>
            <TableHead className={`${HEAD_CELL} text-right`}>{t.common.actions}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody className="whitespace-nowrap">
          {courts.map((court) => (
            <CourtTableRow key={court.id} court={court} complexId={complexId} onEdit={onEdit} onPrices={onPrices} />
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
