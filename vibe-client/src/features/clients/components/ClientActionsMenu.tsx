import { MoreHorizontal, Phone, Ban, ShieldCheck } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { WhatsappIcon } from '@/shared/components/common/WhatsappIcon';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Client } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * WhatsApp, call, block — the three things you can do to a client without
 * opening anything.
 *
 * One menu, used by the card in the list and by the detail drawer's header.
 * There were two of these, and they had already drifted: the card's offered
 * call and block, the drawer's offered WhatsApp as well. Two menus answering
 * the same question is one edit away from disagreeing again.
 */
export function ClientActionsMenu({ client, onBlock }: { client: Client; onBlock: (client: Client) => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={t.common.rowActionsLabel}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem asChild>
          <a href={`https://wa.me/${client.phone.replace(/\D/g, '')}`} target="_blank" rel="noopener noreferrer">
            <WhatsappIcon className="size-3.5" />
            {t.clients.whatsapp}
          </a>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <a href={`tel:${client.phone}`}>
            <Phone className="size-3.5" />
            {t.clients.callClient}
          </a>
        </DropdownMenuItem>
        <DropdownMenuItem
          variant={client.is_blocked ? 'default' : 'destructive'}
          onClick={() => {
            onBlock(client);
          }}
        >
          {client.is_blocked ? <ShieldCheck className="size-3.5" /> : <Ban className="size-3.5" />}
          {client.is_blocked ? t.clients.unblock : t.clients.block}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
