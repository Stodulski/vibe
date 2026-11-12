import type { RefObject } from 'react';
import { AlertTriangle, Users, Loader2 } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { ResultCountAnnouncer } from '@/shared/components/common/ResultCountAnnouncer';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { Skeleton } from '@/shared/components/ui/skeleton';
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
  if (isLoading) return <ClientsSkeleton />;

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
          {isFetchingNextPage && <Loader2 className="text-text-tertiary size-5 animate-spin" />}
        </div>
      )}
    </>
  );
}

/**
 * Loading placeholder shaped like the loaded list at each width: the card grid
 * below `xl` (the grid classes `ClientGrid` uses), the table from `xl` up. Both
 * are in the tree and switched with CSS, which is enough for a placeholder.
 */
function ClientsSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading}>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:hidden">
        {Array.from({ length: 5 }, (_, i) => (
          <ClientCardSkeleton key={i} />
        ))}
      </div>
      <div className="hidden xl:block">
        <SkeletonTable live={false} />
      </div>
    </div>
  );
}

/**
 * Mirrors `ClientCard`: same border, padding and radius, and each bar sits in a
 * box as tall as the text line it stands for, so the card is ~134px like the
 * loaded one.
 */
function ClientCardSkeleton() {
  return (
    <div
      aria-hidden="true"
      className="border-border-subtle bg-bg-subtle flex flex-col items-stretch rounded-2xl border p-4"
    >
      <div className="flex h-5 items-center">
        <Skeleton className="h-4 w-36 rounded-lg" />
      </div>
      <div className="mt-0.5 flex h-4 items-center">
        <Skeleton className="h-3 w-24 rounded-lg" />
      </div>
      <div className="mt-4 grid grid-cols-2 gap-3">
        {[0, 1].map((i) => (
          <div key={i} className="min-w-0">
            <div className="flex h-3.5 items-center">
              <Skeleton className="h-2.5 w-14 rounded-lg" />
            </div>
            <div className="mt-1 flex h-7 items-center">
              <Skeleton className="h-5 w-12 rounded-lg" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
