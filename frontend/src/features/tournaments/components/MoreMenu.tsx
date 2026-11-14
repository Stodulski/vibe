import { MoreHorizontal } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { cn } from '@/shared/lib/utils';

interface MoreMenuItem {
  label: string;
  onSelect: () => void;
  /** Reads as an error colour: the action cancels or removes something. */
  destructive?: boolean;
}

interface MoreMenuProps {
  /** Says what the menu belongs to ("Opciones del torneo"): shown on the button, so two menus never read the same. */
  label: string;
  items: MoreMenuItem[];
}

/** The overflow menu: the actions that are rare or destructive, one interaction away from the screen. */
export function MoreMenu({ label, items }: MoreMenuProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="outline" size="sm" className="rounded-xl">
          <MoreHorizontal className="size-4" aria-hidden="true" />
          {label}
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
