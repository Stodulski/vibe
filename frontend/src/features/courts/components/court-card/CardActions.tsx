import { Pencil, DollarSign, Trash2 } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COURT_CARD_ACTIONS_CLASS } from '../courtLayout';

const t = ES_AR;

interface CardActionsProps {
  courtName: string;
  onEdit: () => void;
  onPrices: () => void;
  onDelete: () => void;
}

/**
 * The card's three actions, all visible.
 *
 * Editar and Precios carry the same weight because they are worth the same:
 * both are routine, either could be the reason the owner opened this page.
 * Eliminar is tertiary and wears no colour of its own — the red belongs to the
 * confirmation dialog, where it describes a decision about to be made.
 *
 * There is no overflow menu: it would hold one item. Below `sm` Eliminar keeps
 * only its icon (its name stays for screen readers) so the three fit a row.
 */
export function CardActions({ courtName, onEdit, onPrices, onDelete }: CardActionsProps) {
  return (
    <div className={COURT_CARD_ACTIONS_CLASS}>
      <Button variant="outline" size="sm" className="flex-1 sm:flex-none" onClick={onEdit}>
        <Pencil className="size-3.5 shrink-0" />
        {t.common.edit}
      </Button>
      <Button variant="outline" size="sm" className="flex-1 sm:flex-none" onClick={onPrices}>
        <DollarSign className="size-3.5 shrink-0" />
        {t.courts.prices}
      </Button>

      <Button
        variant="ghost"
        size="sm"
        className="ml-auto shrink-0"
        aria-label={`${t.common.delete}: ${courtName}`}
        onClick={onDelete}
      >
        <Trash2 className="size-3.5" />
        <span className="hidden sm:inline">{t.common.delete}</span>
      </Button>
    </div>
  );
}
