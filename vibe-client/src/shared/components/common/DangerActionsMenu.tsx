import { MoreHorizontal, Trash2 } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * A destructive action, behind the overflow menu beside a form's submit.
 *
 * Both settings and profile had this as a bordered panel of its own at the
 * bottom of the screen — a heading, a sentence of warning and a button — for
 * something done once in the life of an account, sitting under the thing done
 * every week. And they had already drifted: one was neutral, the other still
 * red before any confirmation.
 *
 * A menu is the friction this step wants: less prominent, one interaction
 * further away, and no red until the confirmation actually asks for it. The
 * warning moves into the dialog, which is where someone reads it.
 */
export function DangerActionsMenu({
  label,
  onSelect,
}: {
  /** The destructive action's own words, e.g. "Eliminar complejo". */
  label: string;
  onSelect: () => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t.common.rowActionsLabel}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={onSelect} className="text-error-text focus:text-error-text">
          <Trash2 className="size-4" />
          {label}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
