import { MoreHorizontal, Ban } from 'lucide-react';
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
 * The event drawer's header menu: the same trigger and destructive item as
 * `BookingDetailMenu`, so every drawer keeps its cancellations in one place.
 */
export function EventDetailMenu({ onCancel }: { onCancel: () => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={t.common.rowActionsLabel}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem variant="destructive" onClick={onCancel}>
          <Ban className="size-3.5" />
          {t.events.cancel.action}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
