import type { ReactNode } from 'react';
import { MoreHorizontal } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';

interface OverflowMenuEntry {
  key: string;
  label: string;
  icon?: ReactNode;
}

export type OverflowMenuItem =
  | (OverflowMenuEntry & { kind: 'action'; onClick: () => void })
  | (OverflowMenuEntry & { kind: 'link'; href: string; external?: boolean })
  | { kind: 'separator'; key: string };

function OverflowMenuItemView({ item }: { item: OverflowMenuItem }) {
  if (item.kind === 'separator') return <DropdownMenuSeparator />;
  if (item.kind === 'action') {
    return (
      <DropdownMenuItem onClick={item.onClick}>
        {item.icon}
        {item.label}
      </DropdownMenuItem>
    );
  }
  return (
    <DropdownMenuItem asChild>
      <a href={item.href} {...(item.external ? { target: '_blank', rel: 'noopener noreferrer' } : {})}>
        {item.icon}
        {item.label}
      </a>
    </DropdownMenuItem>
  );
}

/** The one "⋯" menu of a row: `label` names the trigger for screen readers, and `items` are its actions and links. Renders nothing when there are none. */
export function OverflowMenu({ label, items }: { label: string; items: OverflowMenuItem[] }) {
  if (items.length === 0) return null;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" className="shrink-0" aria-label={label}>
          <MoreHorizontal className="size-4" aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {items.map((item) => (
          <OverflowMenuItemView key={item.key} item={item} />
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
