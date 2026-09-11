import { ClientDetail, BlockClientModal } from '@/features/clients';
import type { useDashboardClientDetail } from '../../hooks/useDashboardClientDetail';

interface ClientInsightsDetailModalsProps {
  detail: ReturnType<typeof useDashboardClientDetail>;
  complexId: string;
}

export function ClientInsightsDetailModals({ detail, complexId }: ClientInsightsDetailModalsProps) {
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
