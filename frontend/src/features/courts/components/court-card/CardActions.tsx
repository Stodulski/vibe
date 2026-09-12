import { Pencil, DollarSign, Trash2, MoreVertical } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface CardActionsProps {
  courtName: string;
  onEdit: () => void;
  onPrices: () => void;
  onDelete: () => void;
}

/**
 * The card's three actions, in two shapes.
 *
 * Editar and Precios carry the same weight because they are worth the same:
 * both are routine, either could be the reason the owner opened this page.
 * Eliminar is tertiary and wears no colour of its own — the red belongs to the
 * confirmation dialog, where it describes a decision about to be made, not to
 * a button sitting on a card doing nothing.
 *
 * Below `sm` there is no room for three controls a thumb can hit, so Eliminar
 * moves behind the overflow. That is not only a space argument: a destructive
 * action one tap further away is a destructive action harder to hit by
 * accident, which is exactly where the extra distance should go.
 */
export function CardActions({ courtName, onEdit, onPrices, onDelete }: CardActionsProps) {
  return (
    <div className="border-border-subtle flex items-center gap-2 border-t px-4 py-3 sm:px-5">
      <Button variant="outline" size="sm" className="flex-1 sm:flex-none" onClick={onEdit}>
        <Pencil className="size-3.5 shrink-0" />
        {t.common.edit}
      </Button>
      <Button variant="outline" size="sm" className="flex-1 sm:flex-none" onClick={onPrices}>
        <DollarSign className="size-3.5 shrink-0" />
        {t.courts.prices}
      </Button>

      <Button variant="ghost" size="sm" className="ml-auto hidden sm:inline-flex" onClick={onDelete}>
        <Trash2 className="size-3.5" />
        {t.common.delete}
      </Button>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            className="shrink-0 sm:hidden"
            aria-label={`${t.common.rowActionsLabel}: ${courtName}`}
          >
            <MoreVertical className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={onDelete}>
            <Trash2 className="size-3.5" />
            {t.common.delete}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
