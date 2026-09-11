import { ClientDetail } from './ClientDetail';
import { BlockClientModal } from './BlockClientModal';
import type { useClientActions } from '@/features/clients/hooks/useClientActions';

interface ClientDrawerModalsProps {
  detail: ReturnType<typeof useClientActions>;
  complexId: string;
}

/**
 * Client drawer + block modal for a caller outside /clients and the
 * dashboard (e.g. the booking detail sheet's client row, from /bookings) —
 * pairs with `useClientActions` the same way `ClientInsightsDetailModals`
 * pairs with `useDashboardClientDetail`, minus the dashboard-only cache
 * invalidation that hook adds on top.
 */
export function ClientDrawerModals({ detail, complexId }: ClientDrawerModalsProps) {
  return (
    <>
      <ClientDetail
        open={detail.detailOpen}
        onClose={() => {
          detail.setDetailOpen(false);
        }}
        client={detail.selectedClient}
        complexId={complexId}
        onBlock={detail.handleBlockClient}
      />
      <BlockClientModal
        open={!!detail.blockClient}
        onClose={() => {
          detail.setBlockClient(null);
        }}
        onConfirm={detail.handleConfirmBlock}
        client={detail.blockClient}
        isLoading={detail.updateClient.isPending}
      />
    </>
  );
}
