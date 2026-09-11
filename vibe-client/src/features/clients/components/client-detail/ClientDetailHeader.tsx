import { DetailSheetHeader } from '@/shared/components/common/DetailSheetHeader';
import { Tag } from '@/shared/components/common/Tag';
import { ClientActionsMenu } from '../ClientActionsMenu';
import type { Client } from '@/shared/types/api.types';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;
interface ClientDetailHeaderProps {
  client: Client;
  fullName: string;
  isBlocked: boolean;
  onBlock: (client: Client) => void;
}

export function ClientDetailHeader({ client, fullName, isBlocked, onBlock }: ClientDetailHeaderProps) {
  return (
    <DetailSheetHeader
      title={fullName}
      subtitle={<span className="score-text">{client.phone}</span>}
      menu={<ClientActionsMenu client={client} onBlock={onBlock} />}
    >
      {isBlocked && (
        <Tag tone="error" className="mt-0.5 w-fit text-micro font-bold uppercase">
          {t.clients.blocked}
        </Tag>
      )}
    </DetailSheetHeader>
  );
}
