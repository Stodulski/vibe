import { MoreHorizontal } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';

interface MoreMenuItem {
  label: string;
  onSelect: () => void;
  /** Reads as an error colour: the action cancels or removes something. */
  destructive?: boolean;
}

interface MoreMenuProps {
  /** Says what the menu belongs to ("Más acciones del torneo"): the visible word alone is the same on every menu. */
  ariaLabel: string;
  items: MoreMenuItem[];
}

/** The "Más" overflow menu: the actions that are rare or destructive, one interaction away from the screen. */
export function MoreMenu({ ariaLabel, items }: MoreMenuProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="outline" size="sm" className="rounded-xl" aria-label={ariaLabel}>
          <MoreHorizontal className="size-4" aria-hidden="true" />
          {ES_AR.tournaments.detail.more}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {items.map((item) => (
          <DropdownMenuItem
            key={item.label}
            onSelect={item.onSelect}
            className={cn(item.destructive && 'text-error-text focus:text-error-text')}
          >
            {item.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
