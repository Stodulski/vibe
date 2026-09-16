import { Pencil, DollarSign, Trash2, MoreHorizontal } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface CourtRowActionsMenuProps {
  courtName: string;
  onEdit: () => void;
  onPrices: () => void;
  onDelete: () => void;
}

/**
 * The row's three actions, behind one `⋯` trigger — the table's reading of
 * `CardActions`. A card has room for two buttons and an overflow; a table row
 * doesn't, so all three live in the same menu here.
 */
export function CourtRowActionsMenu({ courtName, onEdit, onPrices, onDelete }: CourtRowActionsMenuProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={`${t.common.rowActionsLabel}: ${courtName}`}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onEdit}>
          <Pencil className="size-3.5" />
          {t.common.edit}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onPrices}>
          <DollarSign className="size-3.5" />
          {t.courts.prices}
        </DropdownMenuItem>
        <DropdownMenuItem variant="destructive" onClick={onDelete}>
          <Trash2 className="size-3.5" />
          {t.common.delete}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
