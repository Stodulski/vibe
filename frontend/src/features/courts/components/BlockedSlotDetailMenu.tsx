import { MoreHorizontal, LockOpen } from 'lucide-react';
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
 * The blocked slot drawer's header menu: the same trigger and destructive item
 * as `BookingDetailMenu`. Unblocking has no confirmation, as before.
 */
export function BlockedSlotDetailMenu({
  onUnblock,
  disabled,
}: {
  onUnblock: () => void;
  disabled?: boolean | undefined;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={t.common.rowActionsLabel}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem variant="destructive" disabled={disabled ?? false} onClick={onUnblock}>
          <LockOpen className="size-3.5" />
          {t.courts.unblockSlot}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
