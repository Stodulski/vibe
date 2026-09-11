import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { useExitingValue } from '@/shared/hooks/useExitingValue';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Client } from '@/shared/types/api.types';

const t = ES_AR;

interface BlockClientModalProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  client: Client | null;
  isLoading: boolean;
}

export function BlockClientModal({ open, onClose, onConfirm, client, isLoading }: BlockClientModalProps) {
  // The caller drives `open` off this same client (`open={!!blockClient}`) and
  // clears it to close, so a bare `if (!client) return null` unmounted the
  // dialog on the spot instead of letting it fade out.
  const shown = useExitingValue(client);
  if (!shown) return null;

  const isBlocking = !shown.is_blocked;
  const name = `${shown.first_name} ${shown.last_name}`;

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={onConfirm}
      title={isBlocking ? `${t.clients.blockConfirm} ${name}?` : `${t.clients.unblockConfirm} ${name}?`}
      description={isBlocking ? t.clients.blockDescription : t.clients.unblockDescription}
      confirmLabel={isBlocking ? t.clients.block : t.clients.unblock}
      variant={isBlocking ? 'destructive' : 'default'}
      isLoading={isLoading}
    />
  );
}
