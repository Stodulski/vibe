import type { RefObject } from 'react';
import { AlertTriangle, Users, Loader2 } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { ResultCountAnnouncer } from '@/shared/components/common/ResultCountAnnouncer';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { ClientGrid } from '@/features/clients';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Client } from '@/shared/types/api.types';

const t = ES_AR;

interface ClientsContentProps {
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  clients: Client[];
  hasNextPage: boolean | undefined;
  isFetchingNextPage: boolean;
  sentinelRef: RefObject<HTMLDivElement | null>;
  onSelectClient: (client: Client) => void;
  onBlockClient: (client: Client) => void;
}

export function ClientsContent(props: ClientsContentProps) {
  const { isLoading, isError, clients } = props;

  return (
    <>
      {/* Outside every branch below, and mounted before the rows exist: a
          live region that appears together with its text is not reliably
          announced, and the empty branch draws a different tree entirely —
          from inside it, "0 resultados" would never be spoken. `null` while
          the query is loading or failed, so the region is there from the
          first render without announcing a count nobody has yet. */}
      <ResultCountAnnouncer count={isLoading || isError ? null : clients.length} />
      <ClientsBody {...props} />
    </>
  );
}

function ClientsBody({
  isLoading,
  isError,
  onRetry,
  clients,
  hasNextPage,
  isFetchingNextPage,
  sentinelRef,
  onSelectClient,
  onBlockClient,
}: ClientsContentProps) {
  if (isLoading) return <SkeletonTable />;

  if (isError) {
    return (
      <EmptyState
        icon={AlertTriangle}
        title={t.common.loadError}
        description={t.common.loadErrorDescription}
        actionLabel={t.layout.retry}
        onAction={onRetry}
      />
    );
  }

  if (clients.length === 0) {
    return <EmptyState icon={Users} title={t.clients.noClients} description={t.clients.noClientsDescription} />;
  }

  return (
    <>
      <ClientGrid clients={clients} onSelectClient={onSelectClient} onBlockClient={onBlockClient} />

      {hasNextPage && (
        <div ref={sentinelRef} className="flex justify-center py-4">
          {isFetchingNextPage && <Loader2 className="size-5 animate-spin text-text-tertiary" />}
        </div>
      )}
    </>
  );
}
