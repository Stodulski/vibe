import { PageHeader } from '@/shared/components/common/PageHeader';
import { ES_AR } from '@/shared/i18n/es_AR';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ClientDetail, BlockClientModal } from '@/features/clients';
import { ClientsSearchBar } from './clients/ClientsSearchBar';
import { ClientsContent } from './clients/ClientsContent';
import { useClientsPage } from './clients/useClientsPage';

const t = ES_AR;

export default function ClientsPage() {
  usePageTitle(t.clients.title);
  const state = useClientsPage();

  if (!state.selectedComplexId) return null;

  return (
    <div className="animate-fade-in">
      <PageHeader title={t.clients.title} />

      <div className="space-y-2.5 sm:space-y-4">
        <ClientsSearchBar
          searchInput={state.searchInput}
          onSearchChange={state.setSearchInput}
          clientCount={state.clients.length}
        />

        <ClientsContent
          isLoading={state.isLoading}
          isError={state.isError}
          onRetry={() => {
            void state.refetch();
          }}
          clients={state.clients}
          hasNextPage={state.hasNextPage}
          isFetchingNextPage={state.isFetchingNextPage}
          sentinelRef={state.sentinelRef}
          onSelectClient={state.handleSelectClient}
          onBlockClient={state.handleBlockClient}
        />
      </div>

      <ClientDetail
        open={state.detailOpen}
        onClose={() => {
          state.setDetailOpen(false);
        }}
        client={state.selectedClient}
        complexId={state.selectedComplexId}
        onBlock={state.handleBlockClient}
      />

      <BlockClientModal
        open={!!state.blockClient}
        onClose={() => {
          state.setBlockClient(null);
        }}
        onConfirm={state.handleConfirmBlock}
        client={state.blockClient}
        isLoading={state.updateClient.isPending}
      />
    </div>
  );
}
