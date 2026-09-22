import { Wallet } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { ES_AR } from '@/shared/i18n/es_AR';
import { OpenCashSessionDialog } from '../../components/OpenCashSessionDialog';
import { CashSessionHistoryList } from '../../components/CashSessionHistoryList';

const t = ES_AR;

interface ClosedCashViewProps {
  complexId: string;
  openDialogOpen: boolean;
  onOpenDialog: () => void;
  onCloseDialog: () => void;
}

export function ClosedCashView({ complexId, openDialogOpen, onOpenDialog, onCloseDialog }: ClosedCashViewProps) {
  return (
    <div className="space-y-6">
      <EmptyState
        icon={Wallet}
        title={t.cash.closedTitle}
        description={t.cash.closedDescription}
        actionLabel={t.cash.openAction}
        onAction={onOpenDialog}
      />

      <CashSessionHistoryList complexId={complexId} />

      <OpenCashSessionDialog open={openDialogOpen} onClose={onCloseDialog} complexId={complexId} />
    </div>
  );
}
